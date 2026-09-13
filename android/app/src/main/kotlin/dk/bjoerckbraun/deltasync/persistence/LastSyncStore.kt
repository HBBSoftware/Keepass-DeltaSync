// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.persistence

import android.content.Context
import androidx.core.content.edit

/**
 * Hvornår denne enhed sidst synkroniserede, pr. database.
 *
 * Formålet er at kunne svare på "kører det overhovedet?" uden at åbne en
 * server eller et admin-panel. Baggrunds-sync er per definition usynlig, og
 * uden et tidsstempel er en app der virker og en app der har været død i en
 * uge umulige at skelne fra hinanden.
 *
 * **Et tjek uden ændringer tæller også.** Probe-genvejen i
 * [dk.bjoerckbraun.deltasync.worker.SyncWorker] springer den dyre del over
 * når hverken filen eller serveren har ændret sig, og det er langt det
 * almindeligste udfald. Talte kun en faktisk overførsel med, ville
 * tidsstemplet stå stille i dagevis på en rolig database og se ud som om
 * synkroniseringen var gået i stå — altså præcis den bekymring feltet skal
 * fjerne. Gemt er derfor "sidst kørt uden fejl", ikke "sidst overførte noget".
 *
 * Gemmes som et rå millisekund-tal i SharedPreferences, som
 * [SyncProbeStore] ved siden af — det er ét tal pr. database.
 */
class LastSyncStore(context: Context) {

    private val prefs = context.getSharedPreferences(FILE, Context.MODE_PRIVATE)

    /** Tidspunktet for seneste fejlfrie sync, eller null hvis der ikke har været en. */
    fun load(databaseId: String): Long? {
        val at = prefs.getLong(databaseId, -1L)
        return if (at < 0L) null else at
    }

    fun save(databaseId: String, at: Long = System.currentTimeMillis()) {
        prefs.edit { putLong(databaseId, at) }
    }

    /** Ryddes når enheden afmeldes, så en ny tilmelding ikke arver et fremmed tidspunkt. */
    fun clear() {
        prefs.edit { clear() }
    }

    companion object {
        private const val FILE = "deltasync_last_sync"
    }
}
