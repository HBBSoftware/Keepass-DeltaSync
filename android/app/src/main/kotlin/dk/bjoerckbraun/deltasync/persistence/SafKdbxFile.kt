// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.persistence

import android.content.Context
import android.net.Uri
import dk.bjoerckbraun.deltasync.sync.KdbxFile
import java.io.IOException
import java.io.InputStream
import java.io.OutputStream

/**
 * SAF-baseret [KdbxFile]. Læser/skriver via [android.content.ContentResolver]
 * mod en URI brugeren har valgt via Storage Access Framework. URI'en
 * skal være persisteret med ACCESS_PERSISTABLE_URI så app'en stadig kan
 * tilgå filen efter reboot eller proces-død.
 *
 * Atomicity-gap: SAF/DocumentsProvider eksponerer ikke en rename-operation,
 * så [writeAtomic] er reelt truncate-and-write. Vi mitigerer ved at
 * encode hele kdbx'en i hukommelsen først (eller en temp-fil i cache-mappen),
 * og kun derefter åbne ContentResolver-output-streamen — så et fejl i
 * encode-trinet ikke korrumperer den eksisterende fil.
 */
class SafKdbxFile(
    private val context: Context,
    private val uri: Uri,
) : KdbxFile {

    @Throws(IOException::class)
    override fun open(): InputStream {
        return context.contentResolver.openInputStream(uri)
            ?: throw IOException("ContentResolver returned null InputStream for $uri")
    }

    @Throws(IOException::class)
    override fun writeAtomic(action: (OutputStream) -> Unit) {
        // Encode først til en in-memory buffer for at undgå at en encode-
        // fejl truncerer filen før vi har data. For meget store .kdbx-filer
        // (>10 MB) bør vi switche til en temp-fil i cache-dir; for typiske
        // brug-cases (få MB) er memory OK.
        val buffer = java.io.ByteArrayOutputStream()
        action(buffer)

        // Nu skriver vi det færdige indhold til SAF. "wt" mode = write+truncate.
        context.contentResolver.openOutputStream(uri, "wt")?.use { sink ->
            buffer.writeTo(sink)
        } ?: throw IOException("ContentResolver returned null OutputStream for $uri")
    }
}
