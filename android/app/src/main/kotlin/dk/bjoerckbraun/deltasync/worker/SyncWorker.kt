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
import dk.bjoerckbraun.deltasync.persistence.KeystoreTokenStore
import dk.bjoerckbraun.deltasync.persistence.SafKdbxFile
import dk.bjoerckbraun.deltasync.sync.GomobileCryptoSession
import dk.bjoerckbraun.deltasync.sync.Synchronizer
import java.util.concurrent.TimeUnit

/**
 * Baggrunds-worker der kører periodisk sync mod serveren. Trigges af
 * [WorkManager] med en minimum-interval på 15 minutter (Android's
 * lavest tilladte periode for PeriodicWork).
 *
 * Worker'en kører på en baggrunds-coroutine og er IDLE-sikker:
 * SyncEngine'en muterer ikke disk uden netværket har leveret det.
 *
 * Required input data (Data):
 *   - "database_id" : String — server-side database UUID
 *   - "kdbx_path"   : String — absolut sti til den lokale .kdbx-fil
 *   - "passphrase"  : String — kdbx-master-passwordet (KeePassXC's eget)
 *
 * Tabt sync er ikke katastrofisk — næste tick retrier. Vi returnerer
 * Result.retry() ved netværks-fejl og Result.failure() kun ved
 * permanente fejl (revoked token, korrupt kdbx).
 */
class SyncWorker(
    appContext: Context,
    params: WorkerParameters,
) : CoroutineWorker(appContext, params) {

    override suspend fun doWork(): Result {
        val passphrase = inputData.getString(INPUT_PASSPHRASE)
            ?: return Result.failure()

        val tokenStore = KeystoreTokenStore(applicationContext)
        val credentials = tokenStore.load() ?: run {
            Log.w(TAG, "no enrolled device — cannot sync")
            return Result.failure()
        }

        val config = DatabaseConfigStore(applicationContext).load() ?: run {
            Log.w(TAG, "no database configured — cannot sync")
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
            crypto.close()
            Result.success()
        } catch (e: Exception) {
            crypto.close()
            Log.w(TAG, "sync failed; will retry", e)
            Result.retry()
        }
    }

    companion object {
        private const val TAG = "DeltaSyncWorker"
        const val INPUT_PASSPHRASE = "passphrase"
        const val PERIODIC_WORK_NAME = "deltasync-periodic-sync"

        /**
         * Enqueue (eller opdater) den periodiske sync-worker. Worker'en
         * henter selv enrollment + database-config fra de tilsvarende stores;
         * caller'en passer kun masterpasswordet ind via Data.
         *
         * BEMÆRK: passwordet lagres i WorkManager's database i klartekst.
         * For en hardened release skal vi i stedet bruge en biometric/PIN-
         * autentificeret indtastning ved hver kørsel.
         */
        fun enqueuePeriodic(
            context: Context,
            passphrase: String,
            intervalMinutes: Long = 30,
        ) {
            val constraints = Constraints.Builder()
                .setRequiredNetworkType(NetworkType.CONNECTED)
                .build()

            val request = PeriodicWorkRequestBuilder<SyncWorker>(
                intervalMinutes, TimeUnit.MINUTES,
            )
                .setConstraints(constraints)
                .setInputData(
                    androidx.work.Data.Builder()
                        .putString(INPUT_PASSPHRASE, passphrase)
                        .build()
                )
                .build()

            WorkManager.getInstance(context).enqueueUniquePeriodicWork(
                PERIODIC_WORK_NAME,
                ExistingPeriodicWorkPolicy.UPDATE,
                request,
            )
        }
    }
}
