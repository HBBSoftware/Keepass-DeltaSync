// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.persistence

import android.content.Context
import androidx.core.content.edit

/**
 * Et billigt "har filen ændret sig?"-fingeraftryk: sidst-ændret-tidsstempel +
 * filstørrelse. Sammenlignes uden at læse eller dekode filindholdet.
 */
data class FileFingerprint(val lastModified: Long, val size: Long)

/**
 * Persisterer fil-fingeraftrykket fra den seneste vellykkede sync, pr.
 * database. Bruges af probe-genvejen i
 * [dk.bjoerckbraun.deltasync.worker.SyncWorker] (og foreground-sync i
 * MainActivity) til at afgøre om den lokale .kdbx er ændret siden sidst —
 * uden at dekode filen, hvilket koster to Argon2-operationer (kotpass-decode
 * + gomobile-session). Er filen uændret OG serveren ikke har nye changes, er
 * der intet at gøre, og hele den dyre del springes over.
 *
 * Gemmes som rå long-felter (ikke serialiseret JSON) — det er bare to tal.
 */
class SyncProbeStore(context: Context) {

    private val prefs = context.getSharedPreferences(FILE, Context.MODE_PRIVATE)

    /** Det gemte fingeraftryk for [databaseId], eller null hvis intet er gemt. */
    fun load(databaseId: String): FileFingerprint? {
        val mtime = prefs.getLong(key(databaseId, KEY_MTIME), -1L)
        val size = prefs.getLong(key(databaseId, KEY_SIZE), -1L)
        if (mtime < 0L || size < 0L) return null
        return FileFingerprint(mtime, size)
    }

    fun save(databaseId: String, fingerprint: FileFingerprint) {
        prefs.edit {
            putLong(key(databaseId, KEY_MTIME), fingerprint.lastModified)
            putLong(key(databaseId, KEY_SIZE), fingerprint.size)
        }
    }

    fun clear() {
        prefs.edit { clear() }
    }

    private fun key(databaseId: String, field: String) = "$databaseId.$field"

    companion object {
        private const val FILE = "deltasync_sync_probe"
        private const val KEY_MTIME = "mtime"
        private const val KEY_SIZE = "size"
    }
}
