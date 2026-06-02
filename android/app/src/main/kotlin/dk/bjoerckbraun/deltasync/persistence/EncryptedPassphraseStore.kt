// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.persistence

import android.content.Context
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

/**
 * Gemmer kdbx-master-passwordet krypteret med en Android Keystore-forankret
 * master-key (samme mekanisme som [KeystoreTokenStore]). Bruges når brugeren
 * vælger at huske sit password så baggrunds-sync via
 * [dk.bjoerckbraun.deltasync.worker.SyncWorker] kan køre uden manuel
 * indtastning.
 *
 * **Sikkerhedsmodel (bevidst valg):** nøglen kræver IKKE per-brug biometrisk
 * auth. En baggrunds-WorkManager-kørsel har ingen bruger til stede og ingen
 * UI, så en nøgle bundet til en biometrisk prompt ville aldrig kunne
 * dekrypteres i baggrunden. Biometri/device-credential bruges derfor i
 * UI-laget til at gate selve opt-in'en (at slå "husk password" til), ikke til
 * at låse nøglen op ved hver læsning.
 *
 * Konsekvens: passwordet er beskyttet *at-rest* — det ligger krypteret i
 * `<app-data>/shared_prefs/deltasync_passphrase.xml`, kun læsbart af denne
 * app's UID, og forsvinder ved en Keystore-reset. Men en angriber med en
 * ulåst, kørende enhed kan trigge en sync. Det er en accepteret trade-off for
 * at få pålidelig autonom baggrunds-sync; den tidligere kode lagrede til
 * sammenligning passwordet i KLARTEKST i WorkManagers database.
 *
 * Passwordet bindes til den database-UUID det hører til, så et stale password
 * ikke bruges mod en anden database hvis brugeren omkonfigurerer sit setup.
 */
class EncryptedPassphraseStore(context: Context) {

    private val masterKey: MasterKey = MasterKey.Builder(context)
        .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
        .build()

    private val prefs = EncryptedSharedPreferences.create(
        context,
        FILE_NAME,
        masterKey,
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    )

    /** Persister [passphrase] bundet til [databaseId]. */
    fun save(databaseId: String, passphrase: String) {
        prefs.edit()
            .putString(KEY_DATABASE_ID, databaseId)
            .putString(KEY_PASSPHRASE, passphrase)
            .apply()
    }

    /**
     * Returnerer det gemte password hvis et er husket OG det hører til
     * [databaseId]; ellers null. Database-bindingen forhindrer at et password
     * fra et tidligere setup bruges mod en ny database.
     */
    fun load(databaseId: String): String? {
        val storedDb = prefs.getString(KEY_DATABASE_ID, null) ?: return null
        if (storedDb != databaseId) return null
        return prefs.getString(KEY_PASSPHRASE, null)
    }

    /** True hvis et password overhovedet er husket (uanset database). */
    val isRemembered: Boolean
        get() = prefs.getString(KEY_PASSPHRASE, null) != null

    fun clear() {
        prefs.edit().clear().apply()
    }

    companion object {
        private const val FILE_NAME = "deltasync_passphrase"
        private const val KEY_DATABASE_ID = "database_id"
        private const val KEY_PASSPHRASE = "passphrase"
    }
}
