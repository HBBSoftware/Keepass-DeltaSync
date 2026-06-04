// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.persistence

import android.content.Context
import android.net.Uri
import android.provider.DocumentsContract
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

    /**
     * Billigt fil-fingeraftryk (sidst-ændret + størrelse) via en
     * ContentResolver-metadata-query — UDEN at læse eller dekode filindholdet.
     * Bruges af probe-genvejen til at se om filen er ændret siden sidste sync.
     *
     * Returnerer null hvis provideren ikke rapporterer brugbar metadata (nogle
     * cloud-backed DocumentsProviders gør ikke); caller'en falder så tilbage
     * til en fuld sync — en manglende metadata fører aldrig til en forkert
     * skip.
     */
    fun fingerprint(): FileFingerprint? = runCatching {
        context.contentResolver.query(
            uri,
            arrayOf(
                DocumentsContract.Document.COLUMN_LAST_MODIFIED,
                DocumentsContract.Document.COLUMN_SIZE,
            ),
            null, null, null,
        )?.use { cursor ->
            if (!cursor.moveToFirst()) return@use null
            val mtime = if (cursor.isNull(0)) -1L else cursor.getLong(0)
            val size = if (cursor.isNull(1)) -1L else cursor.getLong(1)
            if (mtime < 0L || size < 0L) null else FileFingerprint(mtime, size)
        }
    }.getOrNull()
}
