// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import java.io.IOException
import java.io.InputStream
import java.io.OutputStream
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.StandardCopyOption

/**
 * Abstraktion over hvordan en .kdbx-fil ligger på lager. Skiller
 * [Synchronizer] fra det konkrete persistens-lag — tests bruger en
 * filsystem-baseret impl, Android bruger en SAF (Storage Access Framework)
 * impl der går gennem [android.content.ContentResolver].
 *
 * Implementationer skal sikre at [writeAtomic] er — så vidt muligt på
 * det underliggende storage — atomisk, så en afbrudt sync ikke
 * efterlader en korrupt .kdbx. Filsystem-impl bruger temp-fil + rename;
 * SAF har ikke en direkte mekanisme og må falde tilbage til
 * truncate-and-write (lille window for korruption ved process-kill).
 */
interface KdbxFile {

    /** Åbn en InputStream til at læse det nuværende indhold. */
    @Throws(IOException::class)
    fun open(): InputStream

    /**
     * Skriv nyt indhold via [action]. Implementationen står for at
     * persistere outputtet — typisk ved at action streamer ind i en
     * temp-fil der rename'es over originalen.
     */
    @Throws(IOException::class)
    fun writeAtomic(action: (OutputStream) -> Unit)
}

/**
 * Filsystem-baseret [KdbxFile]. Bruges af tests og af desktop-lignende
 * scenarier hvor vi har direkte path-adgang.
 */
class PathKdbxFile(private val path: Path) : KdbxFile {

    override fun open(): InputStream = Files.newInputStream(path)

    override fun writeAtomic(action: (OutputStream) -> Unit) {
        val tmp = path.resolveSibling("${path.fileName}.tmp")
        try {
            Files.newOutputStream(tmp).use(action)
            Files.move(tmp, path, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE)
        } catch (e: IOException) {
            Files.deleteIfExists(tmp)
            throw e
        }
    }
}
