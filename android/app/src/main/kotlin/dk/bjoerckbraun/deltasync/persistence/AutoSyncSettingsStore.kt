// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.persistence

import android.content.Context
import androidx.core.content.edit

/**
 * Brugerens valg for den periodiske baggrunds-sync — pt. kun intervallet.
 *
 * Selve *om* baggrunds-sync er slået til afledes ikke herfra, men af om der
 * findes et husket password i [EncryptedPassphraseStore] (de to tændes og
 * slukkes altid sammen). Dette store holder kun det interval brugeren har
 * valgt, så valget overlever app-genstart og kan vises i UI'en.
 */
class AutoSyncSettingsStore(context: Context) {

    private val prefs = context.getSharedPreferences(FILE, Context.MODE_PRIVATE)

    /**
     * Det valgte sync-interval i minutter. Default 30. Kun værdier i
     * [ALLOWED_INTERVALS] er meningsfulde (Android tillader minimum 15 min for
     * periodisk WorkManager-arbejde).
     */
    var intervalMinutes: Int
        get() = prefs.getInt(KEY_INTERVAL, DEFAULT_INTERVAL_MINUTES)
        set(value) {
            require(value in ALLOWED_INTERVALS) { "unsupported interval: $value" }
            prefs.edit { putInt(KEY_INTERVAL, value) }
        }

    companion object {
        private const val FILE = "deltasync_autosync"
        private const val KEY_INTERVAL = "interval_minutes"
        const val DEFAULT_INTERVAL_MINUTES = 30
        val ALLOWED_INTERVALS = listOf(15, 30, 60)
    }
}
