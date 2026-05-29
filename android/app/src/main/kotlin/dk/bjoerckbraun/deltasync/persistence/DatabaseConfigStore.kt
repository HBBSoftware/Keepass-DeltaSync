// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.persistence

import android.content.Context
import android.net.Uri
import androidx.core.content.edit

/**
 * Vedvarende mapping mellem brugerens lokale .kdbx-fil (SAF-URI) og den
 * server-side database-UUID den synker mod. Holdes adskilt fra
 * [KeystoreTokenStore] fordi indholdet ikke er højtsensitivt (URI'en og
 * database-UUID'en er begge offentlige sammenlignet med
 * device-private-key'en).
 *
 * Brugeren konfigurerer dette én gang via setup-flowet; herefter kører
 * [dk.bjoerckbraun.deltasync.worker.SyncWorker] med samme config indtil
 * [clear] kaldes.
 */
class DatabaseConfigStore(context: Context) {

    private val prefs = context.getSharedPreferences(FILE, Context.MODE_PRIVATE)

    fun save(uri: Uri, databaseId: String, kdbxName: String) {
        prefs.edit {
            putString(KEY_URI, uri.toString())
            putString(KEY_DATABASE_ID, databaseId)
            putString(KEY_KDBX_NAME, kdbxName)
        }
    }

    fun load(): DatabaseConfig? {
        val uriStr = prefs.getString(KEY_URI, null) ?: return null
        val databaseId = prefs.getString(KEY_DATABASE_ID, null) ?: return null
        val kdbxName = prefs.getString(KEY_KDBX_NAME, "") ?: ""
        return DatabaseConfig(
            uri = Uri.parse(uriStr),
            databaseId = databaseId,
            kdbxName = kdbxName,
        )
    }

    fun clear() {
        prefs.edit { clear() }
    }

    data class DatabaseConfig(
        val uri: Uri,
        val databaseId: String,
        val kdbxName: String,
    )

    companion object {
        private const val FILE = "deltasync_db_config"
        private const val KEY_URI = "uri"
        private const val KEY_DATABASE_ID = "database_id"
        private const val KEY_KDBX_NAME = "kdbx_name"
    }
}
