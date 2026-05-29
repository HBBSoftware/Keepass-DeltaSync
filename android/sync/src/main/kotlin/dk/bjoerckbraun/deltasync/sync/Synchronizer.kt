// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import app.keemobile.kotpass.database.Credentials
import app.keemobile.kotpass.database.KeePassDatabase
import app.keemobile.kotpass.database.decode
import app.keemobile.kotpass.database.encode
import dk.bjoerckbraun.deltasync.api.ApiClient
import java.io.IOException
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.StandardCopyOption

/**
 * Højniveau-orkestrator der pakker hele sync-pipeline'en i én kaldbar
 * enhed:
 *
 *   1. Indlæs `.kdbx` fra disk via [KeePassDatabase.decode].
 *   2. Konverter til [LocalState] via [KotpassLocalStateAdapter.read].
 *   3. Restorér `lastSeq` og `syncedAt` fra [SyncStatePersistence].
 *   4. Kør [SyncEngine.sync] mod serveren.
 *   5. Hvis noget ændredes lokalt: [KotpassLocalStateAdapter.applyToDatabase]
 *      og atomisk skriv det opdaterede `.kdbx` tilbage.
 *   6. Gem opdateret sync-state.
 *
 * En instans repræsenterer ét specifikt `.kdbx` på disk; for at synke
 * flere databaser laver caller'en én Synchronizer per fil.
 *
 * Klassen er IKKE thread-safe — caller skal serialisere [sync]-kald per
 * Synchronizer-instans (fx via en Mutex eller en single-threaded
 * coroutine-dispatcher).
 */
class Synchronizer(
    private val kdbxPath: Path,
    private val credentials: Credentials,
    private val api: ApiClient,
    private val crypto: CryptoSession,
    private val persistence: SyncStatePersistence,
) {

    /**
     * Kør én fuld sync-cyklus for [databaseId] (server-side database UUID).
     * Returnerer en sammenfatning af hvad der skete.
     *
     * Hvis serveren afviste pga. auth-fejl (forkert device-token,
     * adgang trukket tilbage) eller netværket var nede, propagerer
     * undtagelsen — caller'en bør ikke gemme state hvis dette sker
     * (vi gør det selv først *efter* SyncEngine.sync er sluppet ren).
     */
    fun sync(databaseId: String): SyncResult {
        // 1. Indlæs .kdbx
        val db: KeePassDatabase = Files.newInputStream(kdbxPath).use { input ->
            KeePassDatabase.decode(input, credentials)
        }

        // 2. Read til LocalState
        val state = KotpassLocalStateAdapter.read(db)

        // 3. Restore persistens
        val persisted = persistence.load(databaseId)
        state.lastSeq = persisted.lastSeq
        state.syncedAt.putAll(persisted.syncedAt)

        // 4. Kør sync-engine
        val engine = SyncEngine(api, crypto, databaseId)
        val result = engine.sync(state)

        // 5. Hvis vi pullede ændringer fra serveren, opdater den lokale
        // .kdbx atomisk (skriv til temp + rename). Pushed entries krævede
        // ingen lokale ændringer — de var allerede i state.
        if (result.pulledEntries > 0 || result.pulledDeletions > 0) {
            val newDb = KotpassLocalStateAdapter.applyToDatabase(state, db)
            writeAtomic(newDb)
        }

        // 6. Persistér ny sync-state.
        persistence.save(
            databaseId,
            SyncStatePersistence.Persisted(
                lastSeq = state.lastSeq,
                syncedAt = state.syncedAt.toMap(),
            ),
        )

        return result
    }

    /**
     * Skriv [db] til [kdbxPath] atomisk: encode til en temp-fil i samme
     * mappe, derefter rename. Det undgår at en interrupt midt i encode
     * efterlader en korrupt .kdbx — gamle data ligger sikkert indtil
     * rename'en succeeds.
     */
    private fun writeAtomic(db: KeePassDatabase) {
        val tmp = kdbxPath.resolveSibling("${kdbxPath.fileName}.tmp")
        try {
            Files.newOutputStream(tmp).use { out ->
                db.encode(out)
            }
            Files.move(tmp, kdbxPath, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE)
        } catch (e: IOException) {
            Files.deleteIfExists(tmp)
            throw e
        }
    }
}
