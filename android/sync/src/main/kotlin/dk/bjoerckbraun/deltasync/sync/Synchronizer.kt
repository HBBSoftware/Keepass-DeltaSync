// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import app.keemobile.kotpass.database.Credentials
import app.keemobile.kotpass.database.KeePassDatabase
import app.keemobile.kotpass.database.decode
import app.keemobile.kotpass.database.encode
import dk.bjoerckbraun.deltasync.api.ApiClient

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
    private val kdbxFile: KdbxFile,
    private val credentials: Credentials,
    private val api: ApiClient,
    private val crypto: CryptoSession,
    private val persistence: SyncStatePersistence,
    private val progress: SyncProgressListener = SyncProgressListener {},
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
        // 1. Indlæs .kdbx via abstraktionen.
        progress.onProgress(SyncProgressEvent.Loading)
        val db: KeePassDatabase = kdbxFile.open().use { input ->
            KeePassDatabase.decode(input, credentials)
        }

        // 2. Read til LocalState
        val state = KotpassLocalStateAdapter.read(db)

        // 3. Restore persistens
        val persisted = persistence.load(databaseId)
        state.lastSeq = persisted.lastSeq
        state.syncedAt.putAll(persisted.syncedAt)

        // 4. Kør sync-engine
        val engine = SyncEngine(api, crypto, databaseId, progress)
        val result = engine.sync(state)

        // 5. Hvis vi pullede ændringer fra serveren, opdater den lokale
        // .kdbx atomisk (delegeres til KdbxFile-impl).
        if (result.pulledEntries > 0 || result.pulledGroups > 0 || result.pulledDeletions > 0) {
            progress.onProgress(SyncProgressEvent.Writing)
            val newDb = KotpassLocalStateAdapter.applyToDatabase(state, db)
            kdbxFile.writeAtomic { out -> newDb.encode(out) }
        }

        // 6. Persistér ny sync-state.
        persistence.save(
            databaseId,
            SyncStatePersistence.Persisted(
                lastSeq = state.lastSeq,
                syncedAt = state.syncedAt.toMap(),
            ),
        )

        progress.onProgress(SyncProgressEvent.Done)
        return result
    }
}
