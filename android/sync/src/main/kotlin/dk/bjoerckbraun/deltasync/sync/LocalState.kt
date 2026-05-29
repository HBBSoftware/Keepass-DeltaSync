// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import dk.bjoerckbraun.deltasync.canonical.Entry
import kotlinx.datetime.Instant

/**
 * In-memory snapshot af alt SyncEngine'en behøver at vide om databasens
 * lokale tilstand. Det er en data-holder, ikke et persistent storage-lag:
 *
 *  - Mobile-UI laget vil typisk vedligeholde en LocalState som spejling
 *    af kotpass' KeePassDatabase (via [dk.bjoerckbraun.deltasync.canonical.Mapper]),
 *    persistere det til disk, og fodre det til SyncEngine.
 *  - Tests konstruerer LocalState direkte.
 *
 * SyncEngine muterer den i stedet for at returnere nye versioner (Kotlin's
 * immutable-stil er smart men tungere her — vi muterer i et coroutine-
 * eksklusivt scope alligevel).
 */
data class LocalState(
    /** Højeste server_seq vi har set fra `/changes`. Avanceres efter hver sync. */
    var lastSeq: Long = 0,

    /** Aktive entries (ikke-tombstones) keyed by UUID. */
    val entries: MutableMap<String, Entry> = mutableMapOf(),

    /** Tombstones: UUID → deletion mtime. */
    val tombstones: MutableMap<String, Instant> = mutableMapOf(),

    /**
     * Pr.-UUID den højeste mtime vi har "synced" — enten pullet fra serveren
     * eller pushet til den. SyncEngine bruger den til at undgå at gen-pushe
     * entries der ikke er ændret siden sidst.
     *
     * Bemærk: det er IKKE entry-mtimen — det er det mtime-stempel hvor
     * sync-laget sidst registrerede ækvivalens med server. Hvis brugeren
     * editerer entry'en bagefter, vil entry.times.modified vokse > syncedAt
     * og push-fasen vil opdage forskellen.
     */
    val syncedAt: MutableMap<String, Instant> = mutableMapOf(),
)
