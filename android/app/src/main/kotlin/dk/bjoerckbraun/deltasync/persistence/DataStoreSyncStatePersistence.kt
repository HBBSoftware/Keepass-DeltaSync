// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.persistence

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dk.bjoerckbraun.deltasync.sync.SyncStatePersistence
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlinx.datetime.Instant
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

private val Context.syncStateDataStore by preferencesDataStore(name = "sync_state")

/**
 * [SyncStatePersistence]-impl oven på Jetpack DataStore (Preferences).
 *
 * Vi serialiserer hele state pr. database som JSON for at undgå at
 * generere unikke nøgler pr. UUID i syncedAt. Tradeoff: DataStore læser
 * og deserialiserer hele stringen pr. load — for v1 med få entries pr.
 * sync-state er det irrelevant.
 *
 * DataStore er coroutine-først; vi eksponerer dog det synkrone
 * [SyncStatePersistence]-interface og kalder via [runBlocking] internt.
 * Callers af [Synchronizer] kører allerede på en baggrunds-tråd
 * (WorkManager), så det er ufarligt.
 */
class DataStoreSyncStatePersistence(private val context: Context) : SyncStatePersistence {

    private val json = Json {
        ignoreUnknownKeys = true
        encodeDefaults = true
    }

    override fun load(databaseId: String): SyncStatePersistence.Persisted = runBlocking {
        val key = stringPreferencesKey(databaseId)
        val raw = context.syncStateDataStore.data
            .first()[key]
            ?: return@runBlocking SyncStatePersistence.Persisted()
        runCatching {
            val payload = json.decodeFromString(PersistedPayload.serializer(), raw)
            SyncStatePersistence.Persisted(
                lastSeq = payload.lastSeq,
                syncedAt = payload.syncedAt.mapValues { Instant.parse(it.value) },
            )
        }.getOrElse {
            // Korrupt eller skema-drift — drop og start forfra. Med lastSeq=0
            // vil næste sync re-pulle alt, hvilket genskaber en konsistent
            // state.
            SyncStatePersistence.Persisted()
        }
    }

    override fun save(databaseId: String, persisted: SyncStatePersistence.Persisted) = runBlocking {
        val key = stringPreferencesKey(databaseId)
        val payload = PersistedPayload(
            lastSeq = persisted.lastSeq,
            syncedAt = persisted.syncedAt.mapValues { it.value.toString() },
        )
        context.syncStateDataStore.edit { prefs ->
            prefs[key] = json.encodeToString(PersistedPayload.serializer(), payload)
        }
        Unit
    }

    @Serializable
    private data class PersistedPayload(
        val lastSeq: Long,
        val syncedAt: Map<String, String>,
    )
}
