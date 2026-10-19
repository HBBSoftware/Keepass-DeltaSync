// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.persistence

import android.content.Context
import androidx.core.content.edit

/**
 * Hvilken public-key hver modtager havde sidst vi delte med dem:
 * username → KeyID (fuld SHA-256 af nøglen, hex, fra Go-siden).
 *
 * Serveren leverer modtagerens nøgle ved deling, og en kompromitteret server
 * kan levere sin egen. Uden denne hukommelse ville appen ikke kunne se forskel.
 * Kendt og uændret nøgle → del uden spørgsmål; ny eller ændret → brugeren skal
 * sammenligne aftrykket med modtageren. Samme regel som desktop-klientens
 * `known_keys` i config.toml.
 */
class KnownKeyStore(context: Context) {

    private val prefs = context.getSharedPreferences(FILE, Context.MODE_PRIVATE)

    fun load(username: String): String? = prefs.getString(username, null)

    fun save(username: String, keyId: String) {
        prefs.edit { putString(username, keyId) }
    }

    /** Ryddes når enheden afmeldes: en ny tilmelding kan være til en anden server, hvor samme brugernavn er en anden person. */
    fun clear() {
        prefs.edit { clear() }
    }

    companion object {
        private const val FILE = "deltasync_known_keys"
    }
}
