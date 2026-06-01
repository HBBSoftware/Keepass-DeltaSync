// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import dk.bjoerckbraun.deltasync.api.ApiClient
import dk.bjoerckbraun.deltasync.canonical.CanonicalJson
import dk.bjoerckbraun.deltasync.canonical.Entry
import dk.bjoerckbraun.deltasync.canonical.SchemaVersion
import kotlinx.datetime.Instant
import kotlinx.datetime.toJavaInstant
import kotlinx.datetime.toKotlinInstant
import java.util.Base64

/**
 * Orchestrerer pull-derefter-push for en enkelt database. Sync er entry-
 * niveau last-writer-wins via `entry.times.modified`:
 *
 *  - Pull: For hver server-change sammenligner vi server's mtime mod
 *    local mtime. Server nyere → erstat local. Local nyere → behold local
 *    (push-fasen sender den).
 *  - Push: For hver local entry med mtime > syncedAt[uuid] → PUT med
 *    krypteret canonical-blob. For hvert tombstone tilsvarende DELETE.
 *
 * SyncEngine er INGEN thread-safety-grænse — caller skal serialisere kald
 * pr. database (typisk via en Mutex eller en Worker-kø).
 *
 * Funktionen muterer den passede [LocalState]; brug en frisk kopi hvis du vil
 * rollbacke ved fejl.
 */
class SyncEngine(
    private val api: ApiClient,
    private val crypto: CryptoSession,
    private val databaseId: String,
    private val progress: SyncProgressListener = SyncProgressListener {},
) {
    /**
     * Kør én fuld sync-cyklus (pull + push). Returnerer en sammenfatning af
     * hvad der skete; muterer [state] in-place.
     */
    fun sync(state: LocalState): SyncResult {
        val pullResult = pull(state)
        val pushResult = push(state)
        return SyncResult(
            pulledEntries = pullResult.entries,
            pulledDeletions = pullResult.deletions,
            pulledLocalKept = pullResult.localKept,
            pushedEntries = pushResult.entries,
            pushedDeletions = pushResult.deletions,
            newLastSeq = state.lastSeq,
        )
    }

    private data class PullResult(val entries: Int, val deletions: Int, val localKept: Int)

    private fun pull(state: LocalState): PullResult {
        val changes = api.getChanges(databaseId, state.lastSeq)
        val total = changes.entries.size
        var applied = 0
        var deleted = 0
        var kept = 0

        for ((i, change) in changes.entries.withIndex()) {
            progress.onProgress(SyncProgressEvent.Pulling(i + 1, total))
            val serverMtime = parseIso(change.modifiedAt)

            if (change.deleted) {
                val localMtime = state.entries[change.uuid]?.times?.modified
                if (localMtime != null && localMtime > serverMtime) {
                    // Lokal entry er nyere end server's tombstone — keep local,
                    // push-fasen vil sende den til serveren (resurrection).
                    kept++
                } else {
                    state.entries.remove(change.uuid)
                    state.tombstones[change.uuid] = serverMtime
                    state.syncedAt[change.uuid] = serverMtime
                    deleted++
                }
                continue
            }

            // Server har en non-tombstone entry. Dekrypter og parse.
            val blob = Base64.getDecoder().decode(change.blob)
            val entryJson = crypto.decryptEntry(blob)
            val raw = CanonicalJson.decodeFromString(Entry.serializer(), String(entryJson))
            // Server's modified_at metadata er sandheden — overstyr entry's
            // interne times.modified så merge-konflikter løses konsistent
            // (samme rationale som desktop-stien i syncop.go).
            val incoming = raw.copy(times = raw.times.copy(modified = serverMtime))

            val localMtime = state.entries[change.uuid]?.times?.modified
            if (localMtime != null && localMtime > serverMtime) {
                // Lokal er nyere — behold, push-fasen sender den.
                kept++
            } else {
                state.entries[change.uuid] = incoming
                state.tombstones.remove(change.uuid)
                state.syncedAt[change.uuid] = serverMtime
                applied++
            }
        }

        state.lastSeq = changes.currentSeq
        return PullResult(applied, deleted, kept)
    }

    private data class PushResult(val entries: Int, val deletions: Int)

    private fun push(state: LocalState): PushResult {
        var pushedEntries = 0
        var pushedDeletions = 0
        var maxSeq = state.lastSeq

        // Forud-beregn hvad der skal sendes, så progress kan rapportere en
        // meningsfuld total i stedet for at vi tæller med en `continue`-skip.
        val entriesToPush = state.entries.filter { (uuid, entry) ->
            val syncedAt = state.syncedAt[uuid]
            syncedAt == null || syncedAt.isStrictlyBefore(entry.times.modified)
        }
        val tombstonesToPush = state.tombstones.filter { (uuid, at) ->
            val syncedAt = state.syncedAt[uuid]
            syncedAt == null || syncedAt.isStrictlyBefore(at)
        }
        val total = entriesToPush.size + tombstonesToPush.size
        var done = 0

        for ((uuid, entry) in entriesToPush) {
            progress.onProgress(SyncProgressEvent.Pushing(++done, total))
            val mtime = entry.times.modified
            val canonical = if (entry.v == SchemaVersion) entry else entry.copy(v = SchemaVersion)
            val jsonBytes = CanonicalJson.encodeToString(Entry.serializer(), canonical).toByteArray()
            val blob = crypto.encryptEntry(jsonBytes)
            val resp = api.putEntry(databaseId, uuid, blob, mtime.toJavaInstant())
            state.syncedAt[uuid] = mtime
            if (resp.seq > maxSeq) maxSeq = resp.seq
            pushedEntries++
        }

        for ((uuid, at) in tombstonesToPush) {
            progress.onProgress(SyncProgressEvent.Pushing(++done, total))
            val resp = api.deleteEntry(databaseId, uuid, at.toJavaInstant())
            state.syncedAt[uuid] = at
            if (resp.seq > maxSeq) maxSeq = resp.seq
            pushedDeletions++
        }

        state.lastSeq = maxSeq
        return PushResult(pushedEntries, pushedDeletions)
    }

    private fun parseIso(s: String): Instant =
        java.time.Instant.parse(s).toKotlinInstant()

    private fun Instant.isStrictlyBefore(other: Instant): Boolean =
        this < other
}

/** Resultat af én sync-cyklus. */
data class SyncResult(
    /** Entries dekrypteret og applied lokalt. */
    val pulledEntries: Int,
    /** Tombstones modtaget og applied lokalt. */
    val pulledDeletions: Int,
    /** Entries hvor server var ældre end lokal — behold lokal, push den. */
    val pulledLocalKept: Int,
    /** Entries pushet til serveren i denne cyklus. */
    val pushedEntries: Int,
    /** Tombstones pushet til serveren i denne cyklus. */
    val pushedDeletions: Int,
    /** Den nye værdi af LocalState.lastSeq. */
    val newLastSeq: Long,
)
