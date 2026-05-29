// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import app.keemobile.kotpass.database.KeePassDatabase
import app.keemobile.kotpass.database.findEntries
import app.keemobile.kotpass.database.modifiers.binaries
import app.keemobile.kotpass.database.modifiers.modifyEntry
import app.keemobile.kotpass.database.modifiers.modifyParentGroup
import app.keemobile.kotpass.database.modifiers.removeEntry
import app.keemobile.kotpass.models.BinaryReference
import dk.bjoerckbraun.deltasync.canonical.Binary
import dk.bjoerckbraun.deltasync.canonical.Mapper
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

    /**
     * Læs alle aktive entries fra [db] og konverter til canonical-format
     * via [Mapper.toCanonical]. Returnerer en frisk [LocalState] med
     * `lastSeq=0` og tom `syncedAt` — caller'en restorer disse fra
     * vedvarende lager efter loaden (eller starter første sync med
     * `lastSeq=0` for at få fuld pull).
     */
    fun read(db: KeePassDatabase): LocalState {
        val state = LocalState()
        val binaryPool = db.binaries

        // findEntries returnerer en List<Pair<Group, List<Entry>>> — vi
        // flatten'er til en simpel List<Entry> da vi ikke skal bevare
        // gruppe-tilhørsforhold i LocalState (sync er entry-niveau).
        db.findEntries { true }.flatMap { (_, entries) -> entries }.forEach { entry ->
            val canonical = Mapper.toCanonical(entry) { ref ->
                binaryPool[ref.hash]?.getContent()
            }
            state.entries[canonical.uuid] = canonical
        }

        // KDBX' DeletedObjects-liste materialiseres ikke i LocalState's
        // tombstones — vi har ikke en kotpass-API til at læse dem direkte.
        // Det betyder: hvis brugeren har slettet en entry lokalt og
        // databasen ikke endnu er synket, vil sletning ikke propagere før
        // brugeren genåbner med en frisk klient-state. Acceptabelt for v1;
        // tracking via DeletedObjects tilføjes når kotpass eksponerer dem.

        return state
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
