// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

/**
 * Abstraktion over den per-database deriverede entry-encryption-nøgle.
 *
 * Den rigtige implementation er en wrapper rundt om gomobile-bound
 * `deltasync.Session` der lever i `client/mobile/`-Go-pakken. For tests
 * substituerer vi en ren in-memory fake.
 *
 * Implementation skal være thread-safe HVIS der kaldes fra flere
 * coroutines samtidig; den gomobile-baserede implementation er IKKE
 * thread-safe og kræver serialisering på caller-siden.
 */
interface CryptoSession {
    /**
     * Tager canonical-entry JSON og returnerer den krypterede on-wire blob
     * (med format-byte prefix). Throws hvis sessionen er lukket.
     */
    fun encryptEntry(entryJson: ByteArray): ByteArray

    /**
     * Dekrypterer en server-blob og returnerer canonical-entry JSON. Auto-
     * detekterer legacy XML vs canonical-format på Go-siden.
     */
    fun decryptEntry(blob: ByteArray): ByteArray

    /** Zeroer det indlejret keymateriale. Yderligere kald fejler. */
    fun close()
}
