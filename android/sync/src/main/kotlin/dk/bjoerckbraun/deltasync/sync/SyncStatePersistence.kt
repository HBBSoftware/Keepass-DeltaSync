// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import kotlinx.datetime.Instant

/**
 * Vedvarende lager for sync-state mellem app-starter. Det indeholder kun
 * de felter SyncEngine'en har brug for at undgå at re-sende eller re-
 * modtage entries der allerede er synket: lastSeq (højeste server_seq vi
 * har set) og syncedAt (per-UUID mtime vi har bekræftet med serveren).
 *
 * Lokale entry-data ligger i selve .kdbx-filen og skal IKKE duplikeres
 * her — det er kun den ephemeral sync-tracking-metadata der har brug for
 * persistens.
 *
 * Android-impl bruger typisk DataStore / SharedPreferences med JSON-
 * encoded indhold. Tests bruger en in-memory impl.
 */
interface SyncStatePersistence {

    /**
     * Indlæs den senest gemte state for [databaseId]. Returnerer en frisk
     * [Persisted] med `lastSeq=0` og tomt `syncedAt`-map hvis intet er
     * gemt endnu (første sync efter app-install eller fresh enroll).
     */
    fun load(databaseId: String): Persisted

    /**
     * Skriv den opdaterede state for [databaseId]. Kald'es efter hver
     * succesfuld [Synchronizer.sync].
     */
    fun save(databaseId: String, persisted: Persisted)

    /**
     * Snapshot af det vedvarende sync-state-data.
     */
    data class Persisted(
        val lastSeq: Long = 0,
        val syncedAt: Map<String, Instant> = emptyMap(),
    )
}

/**
 * Thread-safe in-memory impl af [SyncStatePersistence]. Bruges af tests
 * og som default for Android-impl der ikke har persisteret noget endnu
 * (fx før første save).
 */
class InMemorySyncStatePersistence : SyncStatePersistence {
    private val store = mutableMapOf<String, SyncStatePersistence.Persisted>()

    @Synchronized
    override fun load(databaseId: String): SyncStatePersistence.Persisted =
        store[databaseId] ?: SyncStatePersistence.Persisted()

    @Synchronized
    override fun save(databaseId: String, persisted: SyncStatePersistence.Persisted) {
        store[databaseId] = persisted
    }
}
