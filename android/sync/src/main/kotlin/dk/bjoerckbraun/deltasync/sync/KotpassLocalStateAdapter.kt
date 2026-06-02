// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import app.keemobile.kotpass.database.KeePassDatabase
import app.keemobile.kotpass.database.findEntries
import app.keemobile.kotpass.database.modifiers.binaries
import app.keemobile.kotpass.database.modifiers.modifyEntry
import app.keemobile.kotpass.database.modifiers.modifyParentGroup
import app.keemobile.kotpass.database.modifiers.removeEntry
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
        collectEntries(db.content.group, recycleBinUuid, binaryPool, state)

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
    private fun collectEntries(
        group: Group,
        recycleBinUuid: UUID?,
        binaryPool: Map<ByteString, BinaryData>,
        state: LocalState,
    ) {
        val inRecycleBin = recycleBinUuid != null && group.uuid == recycleBinUuid
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
            }
            state.entries[canonical.uuid] = canonical
        }
        for (child in group.groups) {
            collectEntries(child, recycleBinUuid, binaryPool, state)
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

        // 1. Slet tombstones.
        for (uuidString in state.tombstones.keys) {
            val uuid = runCatching { UUID.fromString(uuidString) }.getOrNull() ?: continue
            db = db.removeEntry(uuid)
        }

        // 2-3. Upsert entries. Vi snapshot'er db's eksisterende UUIDs én
        // gang og forgrener på det — modifyParentGroup tilføjer nye,
        // modifyEntry opdaterer gamle.
        val existingUuids = db.findEntries { true }
            .flatMap { (_, entries) -> entries }
            .map { it.uuid.toString() }
            .toSet()

        for ((uuidString, canonicalEntry) in state.entries) {
            val uuid = runCatching { UUID.fromString(uuidString) }.getOrNull() ?: continue

            // Binary store callback: tilføj data til pool og returnér ref.
            // Dette muterer ikke db direkte; kotpass forventer at vi har
            // pool'en opdateret før entry'en indsættes. For v1 bruger vi
            // hashen som ID og lader kotpass' encoder fange duplikationer.
            val binaryStore: (Binary) -> BinaryReference = { binary ->
                // Hash bytes så vi får en konsistent reference. Brug SHA-256
                // ligesom KDBX' egen pool-deduplikering.
                val md = java.security.MessageDigest.getInstance("SHA-256")
                val hash = md.digest(binary.data)
                BinaryReference(
                    hash = okio.ByteString.of(*hash),
                    name = binary.name,
                )
            }

            val kotpassEntry = Mapper.toKotpass(canonicalEntry, binaryStore)

            db = if (uuidString in existingUuids) {
                db.modifyEntry(uuid) { kotpassEntry }
            } else {
                db.modifyParentGroup {
                    copy(entries = entries + kotpassEntry)
                }
            }
        }

        // Binaries der refereres af de tilføjede entries skal også være i
        // pool'en. For v1 stoler vi på at applikations-laget allerede har
        // sørget for det via kotpass.modifyBinaries; LocalState's Binary-
        // bytes lever ikke nødvendigvis i pool'en før encode.
        // TODO: når vi har en konkret use-case kan vi tilføje pool-sync her.

        return db
    }
}
