// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.worker

import android.content.Context
import android.util.Log
import androidx.work.CoroutineWorker
import androidx.work.NetworkType
import androidx.work.Constraints
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import app.keemobile.kotpass.cryptography.EncryptedValue
import app.keemobile.kotpass.database.Credentials
import dk.bjoerckbraun.deltasync.api.ApiClient
import dk.bjoerckbraun.deltasync.persistence.DatabaseConfigStore
import dk.bjoerckbraun.deltasync.persistence.DataStoreSyncStatePersistence
import dk.bjoerckbraun.deltasync.persistence.EncryptedPassphraseStore
import dk.bjoerckbraun.deltasync.persistence.KeystoreTokenStore
import dk.bjoerckbraun.deltasync.persistence.SafKdbxFile
import dk.bjoerckbraun.deltasync.sync.GomobileCryptoSession
import dk.bjoerckbraun.deltasync.sync.Synchronizer
import java.io.IOException
import java.util.concurrent.TimeUnit

/**
 * Baggrunds-worker der kører periodisk sync mod serveren. Trigges af
 * [WorkManager] med en interval på 30 minutter (Android's lavest tilladte
 * periode er 15 min).
 *
 * Worker'en henter selv alt den behøver fra de tre stores — der sendes
 * INGEN hemmeligheder gennem WorkManagers `Data` (den ville ligge i
 * klartekst i WorkManagers DB):
 *   - device-credentials fra [KeystoreTokenStore]
 *   - database-config fra [DatabaseConfigStore]
 *   - kdbx-master-passwordet fra [EncryptedPassphraseStore] (Keystore-
 *     krypteret; lagt der da brugeren slog "husk password" til)
 *
 * Er ingen af de tre på plads (ikke enrolled, ingen database, eller intet
 * husket password), er der intet at gøre i baggrunden → [Result.failure].
 *
 * Fejl-klassificering: netværks-fejl ([IOException]) → [Result.retry] (næste
 * tick prøver igen). Alt andet (forkert password, korrupt kdbx, trukket
 * adgang) er permanent → vi rydder det gemte password, aflyser den
 * periodiske sync, og returnerer [Result.failure] så vi ikke looper på en
 * fejl brugeren skal gribe ind i.
 */
class SyncWorker(
    appContext: Context,
    params: WorkerParameters,
) : CoroutineWorker(appContext, params) {

    override suspend fun doWork(): Result {
        val tokenStore = KeystoreTokenStore(applicationContext)
        val credentials = tokenStore.load() ?: run {
            Log.w(TAG, "no enrolled device — cannot sync")
            return Result.failure()
        }

        val config = DatabaseConfigStore(applicationContext).load() ?: run {
            Log.w(TAG, "no database configured — cannot sync")
            return Result.failure()
        }

        val passphraseStore = EncryptedPassphraseStore(applicationContext)
        val passphrase = passphraseStore.load(config.databaseId) ?: run {
            Log.w(TAG, "no remembered passphrase — background sync not enabled")
            return Result.failure()
        }

        val kdbxCredentials = Credentials.from(EncryptedValue.fromString(passphrase))

        val api = ApiClient(
            baseUrl = credentials.serverUrl,
            deviceToken = credentials.deviceToken,
        )
        val crypto = GomobileCryptoSession.open(
            password = passphrase.toByteArray(),
            databaseId = config.databaseId,
        )

        val persistence = DataStoreSyncStatePersistence(applicationContext)
        val synchronizer = Synchronizer(
            kdbxFile = SafKdbxFile(applicationContext, config.uri),
            credentials = kdbxCredentials,
            api = api,
            crypto = crypto,
            persistence = persistence,
        )

        return try {
            val result = synchronizer.sync(config.databaseId)
            Log.i(TAG, "sync ok: $result")
            Result.success()
        } catch (e: IOException) {
            // Netværk nede / server midlertidigt utilgængelig — prøv igen.
            Log.w(TAG, "sync failed (network); will retry", e)
            Result.retry()
        } catch (e: Exception) {
            // Permanent: forkert password, korrupt fil, eller trukket adgang.
            // Et husket-men-forkert password må ikke loope i baggrunden —
            // ryd det og stop den periodiske sync. Brugeren slår den til igen
            // (og taster password på ny) fra app'en.
            Log.w(TAG, "sync failed (permanent); clearing remembered passphrase", e)
            passphraseStore.clear()
            cancelPeriodic(applicationContext)
            Result.failure()
        } finally {
            crypto.close()
        }
    }

    companion object {
        private const val TAG = "DeltaSyncWorker"
        const val PERIODIC_WORK_NAME = "deltasync-periodic-sync"

        /**
         * Enqueue (eller opdater) den periodiske sync-worker. Worker'en henter
         * selv enrollment, database-config og det gemte password fra de
         * tilsvarende stores — caller'en passer ingen hemmeligheder ind.
         *
         * Kald [cancelPeriodic] for at slå baggrunds-sync fra igen.
         */
        fun enqueuePeriodic(
            context: Context,
            intervalMinutes: Long = 30,
        ) {
            val constraints = Constraints.Builder()
                .setRequiredNetworkType(NetworkType.CONNECTED)
                .build()

            val request = PeriodicWorkRequestBuilder<SyncWorker>(
                intervalMinutes, TimeUnit.MINUTES,
            )
                .setConstraints(constraints)
                .build()

            WorkManager.getInstance(context).enqueueUniquePeriodicWork(
                PERIODIC_WORK_NAME,
                ExistingPeriodicWorkPolicy.UPDATE,
                request,
            )
        }

        /** Aflys den periodiske baggrunds-sync. */
        fun cancelPeriodic(context: Context) {
            WorkManager.getInstance(context).cancelUniqueWork(PERIODIC_WORK_NAME)
        }
    }
}
