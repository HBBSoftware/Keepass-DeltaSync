// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import app.keemobile.kotpass.database.KeePassDatabase
import app.keemobile.kotpass.database.findEntries
import app.keemobile.kotpass.database.modifiers.binaries
import app.keemobile.kotpass.database.modifiers.modifyEntry
import app.keemobile.kotpass.database.modifiers.modifyGroup
import app.keemobile.kotpass.database.modifiers.modifyParentGroup
import app.keemobile.kotpass.database.modifiers.moveEntry
import app.keemobile.kotpass.database.modifiers.moveGroup
import app.keemobile.kotpass.database.modifiers.removeEntry
import app.keemobile.kotpass.database.modifiers.removeGroup
import app.keemobile.kotpass.models.BinaryData
import app.keemobile.kotpass.models.BinaryReference
import app.keemobile.kotpass.models.Group
import dk.bjoerckbraun.deltasync.canonical.Binary
import dk.bjoerckbraun.deltasync.canonical.Mapper
import kotlinx.datetime.Instant
import kotlinx.datetime.toKotlinInstant
import okio.ByteString
import java.util.UUID

/**
 * Spejler en kotpass [KeePassDatabase] mod vores [LocalState], så
 * [SyncEngine] kan operere på den faktiske .kdbx-fil. Lever som et
 * stateless object — al konvertering går gennem [Mapper] og kotpass'
 * egne immutable modify-funktioner.
 *
 * Brug:
 *
 *   ```kotlin
 *   var db = KeePassDatabase.decode(stream, credentials)
 *   val state = KotpassLocalStateAdapter.read(db)
 *
 *   // ... SyncEngine.sync(state) muterer state ...
 *
 *   db = KotpassLocalStateAdapter.applyToDatabase(state, db)
 *   db.encode(outputStream)
 *   ```
 *
 * KeePassDatabase er immutable; alle modifikationer returnerer en ny
 * instans. Caller'en holder fast i den seneste.
 */
object KotpassLocalStateAdapter {

    /** Null-UUID (00000000-…) — KDBX' sentinel for "ingen recycle-bin udpeget". */
    private val ZERO_UUID = UUID(0L, 0L)

    /** Sidste-udvej deletion-time når et tombstone mangler tidsstempler. */
    private val EPOCH = Instant.fromEpochMilliseconds(0)

    /**
     * Læs alle aktive entries fra [db] og konverter til canonical-format
     * via [Mapper.toCanonical]. Returnerer en frisk [LocalState] med
     * `lastSeq=0` og tom `syncedAt` — caller'en restorer disse fra
     * vedvarende lager efter loaden (eller starter første sync med
     * `lastSeq=0` for at få fuld pull).
     *
     * Sletninger materialiseres som tombstones på to måder, identisk med
     * desktop-klientens `ParseExport` (`client/internal/kdbx/xml.go`):
     *
     *  1. **Recycle-bin-synthesis:** hvis recycle bin er aktiveret og en
     *     gruppe er udpeget, behandles entries der ligger DIREKTE i den
     *     gruppe som sletninger med `DeletedAt = LocationChanged` (det
     *     tidspunkt entry'en blev flyttet i papirkurven). Det er den
     *     almindelige sti — KeePass' standard er at "slet" flytter til
     *     papirkurven frem for at fjerne entry'en helt.
     *  2. **DeletedObjects:** permanente sletninger (papirkurv tømt, eller
     *     papirkurv deaktiveret) lever i KDBX' `<DeletedObjects>`-liste med
     *     UUID + deletion-time.
     *
     * Trade-off (samme som desktop): undelete — at flytte en entry UD af
     * papirkurven igen — propagerer ikke, da entry'en allerede er slettet på
     * serveren. Entries i UNDERgrupper af papirkurven behandles som aktive
     * (kun direkte børn synthesizes), igen for paritet med desktop.
     */
    fun read(db: KeePassDatabase): LocalState {
        val state = LocalState()
        val binaryPool = db.binaries
        val recycleBinUuid = activeRecycleBinUuid(db)

        // Vi traverserer gruppe-træet selv frem for at bruge kotpass'
        // findEntries: den filtrerer recycle-bin-entries fra baseret på
        // meta.recycleBinUuid, så vi ville aldrig se dem og kunne ikke
        // synthesize tombstones. Egen rekursion giver fuld kontrol og
        // matcher desktop'ens collectEntries 1:1.
        collectTree(db.content.group, isRoot = true, recycleBinUuid, binaryPool, state)

        // DeletedObjects: permanente sletninger. En aktiv entry vinder over
        // et stale tombstone (resurrection), så vi springer UUIDs over der
        // allerede optræder som levende entry.
        for (deleted in db.content.deletedObjects) {
            val uuid = deleted.id.toString()
            if (state.entries.containsKey(uuid)) continue
            state.tombstones[uuid] = deleted.deletionTime.toKotlinInstant()
        }

        return state
    }

    /**
     * Walk'er [group]-træet rekursivt. Entries i den gruppe hvis UUID matcher
     * [recycleBinUuid] synthesizes som tombstones (`DeletedAt =
     * LocationChanged`); alle øvrige bliver aktive entries. Bemærk at
     * `inRecycleBin` genberegnes pr. gruppe, så entries i UNDERgrupper af
     * papirkurven IKKE synthesizes — kun direkte børn — identisk med desktop.
     */
    private fun collectTree(
        group: Group,
        isRoot: Boolean,
        recycleBinUuid: UUID?,
        binaryPool: Map<ByteString, BinaryData>,
        state: LocalState,
    ) {
        val inRecycleBin = recycleBinUuid != null && group.uuid == recycleBinUuid

        // Reference som entries i denne gruppe + dens børn peger på: "" for
        // Root (sentinel), ellers gruppens UUID. Matcher desktop's collectTree.
        val thisRef = if (isRoot) "" else group.uuid.toString()

        for (entry in group.entries) {
            if (inRecycleBin) {
                // LocationChanged = tidspunktet entry'en blev flyttet i
                // papirkurven; fallback til LastModificationTime, derefter
                // epoch (defensivt — kotpass udfylder normalt begge).
                val deletedAt = (entry.times?.locationChanged
                    ?: entry.times?.lastModificationTime)
                    ?.toKotlinInstant() ?: EPOCH
                state.tombstones[entry.uuid.toString()] = deletedAt
                continue
            }
            val canonical = Mapper.toCanonical(entry) { ref ->
                binaryPool[ref.hash]?.getContent()
            }.copy(parentGroup = thisRef)
            state.entries[canonical.uuid] = canonical
        }

        for (child in group.groups) {
            val childIsRecycleBin = recycleBinUuid != null && child.uuid == recycleBinUuid
            // Emit child som synket gruppe — men ikke recycle-bin'en eller dens
            // undertræ (lokalt/slettet). Root selv emittes aldrig.
            if (!inRecycleBin && !childIsRecycleBin) {
                val cg = Mapper.groupToCanonical(child, thisRef)
                state.groups[cg.uuid] = cg
            }
            collectTree(child, isRoot = false, recycleBinUuid, binaryPool, state)
        }
    }

    /**
     * Recycle-bin-gruppens UUID hvis papirkurven er aktiveret OG en gruppe
     * faktisk er udpeget; ellers null (ingen synthesis). Spejler desktop's
     * `activeRecycleBinUUID`.
     */
    private fun activeRecycleBinUuid(db: KeePassDatabase): UUID? {
        val meta = db.content.meta
        if (!meta.recycleBinEnabled) return null
        val uuid = meta.recycleBinUuid ?: return null
        return if (uuid == ZERO_UUID) null else uuid
    }

    /**
     * Anvend [state]'s ændringer på [db] og returnér den modificerede
     * database. Operationen er idempotent og fejler stilt hvis state
     * indeholder noget der ikke kan udtrykkes i kotpass-formatet (typisk
     * UUID parse-fejl).
     *
     * Strategi:
     *
     *  1. For hver tombstone i state: `db.removeEntry(uuid)`.
     *  2. For hver entry i state der ALLEREDE er i db: `db.modifyEntry`.
     *  3. For hver entry i state der IKKE er i db: tilføj til Root-gruppen
     *     via [KeePassDatabase.modifyParentGroup].
     *
     * Binaries: [binaryStore] tilføjes til poolen via [Mapper.toKotpass]'s
     * callback. Default-implementationen hash'er og indsætter i pool'en —
     * men placeholders bruges hvis poolen kræver en spec-specifik
     * [app.keemobile.kotpass.models.BinaryData]-konstruktion. Real-world
     * apps overrider denne callback med kotpass' egen
     * [KeePassDatabase.modifyBinaries].
     */
    fun applyToDatabase(state: LocalState, original: KeePassDatabase): KeePassDatabase {
        var db = original
        val rootUuid = db.content.group.uuid

        // Snapshot start-tilstanden: eksisterende entry-/gruppe-UUIDs + hvert
        // objekts nuværende forælder (til flytte-detektion). Vi forgrener på
        // start-tilstanden; modify/move/add-kaldene er idempotente nok.
        val existingEntryUuids = db.findEntries { true }
            .flatMap { (_, entries) -> entries }
            .map { it.uuid.toString() }
            .toSet()
        val existingGroupUuids = mutableSetOf<String>()
        val currentParent = mutableMapOf<String, String>()
        collectExisting(db.content.group, isRoot = true, existingGroupUuids, currentParent)

        // "" eller ukendt parent → Root (sentinel-mapping, samme som desktop).
        fun resolveParent(p: String): UUID {
            if (p.isNotEmpty() && (p in existingGroupUuids || state.groups.containsKey(p))) {
                return runCatching { UUID.fromString(p) }.getOrNull() ?: rootUuid
            }
            return rootUuid
        }

        // 1. Slet tombstones — gruppe ELLER entry (fælles UUID-rum).
        for (uuidString in state.tombstones.keys) {
            val uuid = runCatching { UUID.fromString(uuidString) }.getOrNull() ?: continue
            db = if (uuidString in existingGroupUuids) db.removeGroup(uuid) else db.removeEntry(uuid)
        }

        // 2. Upsert grupper — forældre før børn (topologisk), så en ny
        //    undergruppe kan tilføjes til en parent der allerede findes.
        val created = mutableSetOf<String>()
        val pending = state.groups.values.toMutableList()
        var progress = true
        while (pending.isNotEmpty() && progress) {
            progress = false
            val iter = pending.iterator()
            while (iter.hasNext()) {
                val g = iter.next()
                val uuid = runCatching { UUID.fromString(g.uuid) }.getOrNull()
                if (uuid == null) { iter.remove(); continue }
                // Kan parent resolves nu? ("" / eksisterende / allerede oprettet /
                // ukendt→root). Ellers vent på at parent oprettes i en senere runde.
                val parentReady = g.parentGroup.isEmpty() ||
                    g.parentGroup in existingGroupUuids ||
                    g.parentGroup in created ||
                    !state.groups.containsKey(g.parentGroup)
                if (!parentReady) continue
                val parentUuid = resolveParent(g.parentGroup)
                val kp = Mapper.groupToKotpass(g)
                if (g.uuid in existingGroupUuids) {
                    // Opdater felter — BEVAR eksisterende børn (copy på den
                    // nuværende gruppe rører ikke entries/groups).
                    db = db.modifyGroup(uuid) {
                        copy(name = kp.name, notes = kp.notes, icon = kp.icon,
                            customIconUuid = kp.customIconUuid, times = kp.times)
                    }
                    if (uuid != rootUuid && currentParent[g.uuid] != parentUuid.toString()) {
                        db = db.moveGroup(uuid, parentUuid)
                    }
                } else {
                    db = addChildGroup(db, parentUuid, rootUuid, kp)
                }
                created.add(g.uuid)
                iter.remove()
                progress = true
            }
        }
        // Cyklus/uopnåelige rest-grupper: tilføj defensivt under Root.
        for (g in pending) {
            val uuid = runCatching { UUID.fromString(g.uuid) }.getOrNull() ?: continue
            if (g.uuid in existingGroupUuids) continue
            db = addChildGroup(db, rootUuid, rootUuid, Mapper.groupToKotpass(g))
        }

        // 3. Upsert entries i deres parent-gruppe (flyt eksisterende ved behov).
        for ((uuidString, canonicalEntry) in state.entries) {
            val uuid = runCatching { UUID.fromString(uuidString) }.getOrNull() ?: continue
            val binaryStore: (Binary) -> BinaryReference = { binary ->
                val md = java.security.MessageDigest.getInstance("SHA-256")
                BinaryReference(hash = okio.ByteString.of(*md.digest(binary.data)), name = binary.name)
            }
            val kotpassEntry = Mapper.toKotpass(canonicalEntry, binaryStore)
            val parentUuid = resolveParent(canonicalEntry.parentGroup)
            if (uuidString in existingEntryUuids) {
                db = db.modifyEntry(uuid) { kotpassEntry }
                if (currentParent[uuidString] != parentUuid.toString()) {
                    db = db.moveEntry(uuid, parentUuid)
                }
            } else {
                db = if (parentUuid == rootUuid) {
                    db.modifyParentGroup { copy(entries = entries + kotpassEntry) }
                } else {
                    db.modifyGroup(parentUuid) { copy(entries = entries + kotpassEntry) }
                }
            }
        }

        return db
    }

    /**
     * Walk'er det eksisterende træ og opsamler gruppe-UUIDs (ekskl. Root) +
     * hvert barns (entry/gruppe) nuværende forælder-UUID — bruges til
     * flytte-detektion i [applyToDatabase].
     */
    private fun collectExisting(
        group: Group,
        isRoot: Boolean,
        groupUuids: MutableSet<String>,
        currentParent: MutableMap<String, String>,
    ) {
        val thisUuid = group.uuid.toString()
        if (!isRoot) groupUuids.add(thisUuid)
        for (entry in group.entries) currentParent[entry.uuid.toString()] = thisUuid
        for (child in group.groups) {
            currentParent[child.uuid.toString()] = thisUuid
            collectExisting(child, isRoot = false, groupUuids, currentParent)
        }
    }

    /** Tilføj [child] som undergruppe til [parentUuid] (Root via modifyParentGroup). */
    private fun addChildGroup(
        db: KeePassDatabase,
        parentUuid: UUID,
        rootUuid: UUID,
        child: Group,
    ): KeePassDatabase =
        if (parentUuid == rootUuid) {
            db.modifyParentGroup { copy(groups = groups + child) }
        } else {
            db.modifyGroup(parentUuid) { copy(groups = groups + child) }
        }
}
