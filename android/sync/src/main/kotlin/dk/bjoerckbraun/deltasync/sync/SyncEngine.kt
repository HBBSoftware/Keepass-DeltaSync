// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import dk.bjoerckbraun.deltasync.api.ApiClient
import dk.bjoerckbraun.deltasync.canonical.CanonicalJson
import dk.bjoerckbraun.deltasync.canonical.Entry
import dk.bjoerckbraun.deltasync.canonical.Group
import dk.bjoerckbraun.deltasync.canonical.GroupSchemaVersion
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
            pulledGroups = pullResult.groups,
            pulledDeletions = pullResult.deletions,
            pulledLocalKept = pullResult.localKept,
            pushedEntries = pushResult.entries,
            pushedGroups = pushResult.groups,
            pushedDeletions = pushResult.deletions,
            newLastSeq = state.lastSeq,
        )
    }

    private data class PullResult(val entries: Int, val groups: Int, val deletions: Int, val localKept: Int)

    private fun pull(state: LocalState): PullResult {
        val changes = api.getChanges(databaseId, state.lastSeq, includeGroups = true)
        val total = changes.entries.size
        var applied = 0
        var appliedGroups = 0
        var deleted = 0
        var kept = 0

        for ((i, change) in changes.entries.withIndex()) {
            progress.onProgress(SyncProgressEvent.Pulling(i + 1, total))
            val serverMtime = parseIso(change.modifiedAt)

            if (change.deleted) {
                // Tombstone kan være entry ELLER gruppe (fælles UUID-rum).
                val localMtime = state.entries[change.uuid]?.times?.modified
                    ?: state.groups[change.uuid]?.times?.modified
                if (localMtime != null && localMtime > serverMtime) {
                    // Lokal er nyere end server's tombstone — keep local,
                    // push-fasen vil sende den til serveren (resurrection).
                    kept++
                } else {
                    state.entries.remove(change.uuid)
                    state.groups.remove(change.uuid)
                    state.tombstones[change.uuid] = serverMtime
                    state.syncedAt[change.uuid] = serverMtime
                    deleted++
                }
                continue
            }

            // Gruppe-objekt (v4 group-sync): dekrypter, LWW på times.modified.
            if (change.kind == 2) {
                val blob = Base64.getDecoder().decode(change.blob)
                val groupJson = crypto.decryptGroup(blob)
                val raw = CanonicalJson.decodeFromString(Group.serializer(), String(groupJson))
                val incoming = raw.copy(times = raw.times.copy(modified = serverMtime))
                val localMtime = state.groups[change.uuid]?.times?.modified
                if (localMtime != null && localMtime > serverMtime) {
                    kept++
                } else {
                    state.groups[change.uuid] = incoming
                    state.tombstones.remove(change.uuid)
                    state.syncedAt[change.uuid] = serverMtime
                    appliedGroups++
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
        return PullResult(applied, appliedGroups, deleted, kept)
    }

    private data class PushResult(val entries: Int, val groups: Int, val deletions: Int)

    private fun push(state: LocalState): PushResult {
        var pushedEntries = 0
        var pushedGroups = 0
        var pushedDeletions = 0
        var maxSeq = state.lastSeq

        // Forud-beregn hvad der skal sendes, så progress kan rapportere en
        // meningsfuld total i stedet for at vi tæller med en `continue`-skip.
        val groupsToPush = state.groups.filter { (uuid, g) ->
            val syncedAt = state.syncedAt[uuid]
            syncedAt == null || syncedAt.isStrictlyBefore(g.times.modified)
        }
        val entriesToPush = state.entries.filter { (uuid, entry) ->
            val syncedAt = state.syncedAt[uuid]
            syncedAt == null || syncedAt.isStrictlyBefore(entry.times.modified)
        }
        val tombstonesToPush = state.tombstones.filter { (uuid, at) ->
            val syncedAt = state.syncedAt[uuid]
            syncedAt == null || syncedAt.isStrictlyBefore(at)
        }
        val total = groupsToPush.size + entriesToPush.size + tombstonesToPush.size
        var done = 0

        // Grupper først, så parent-grupper findes server-side før entries der
        // peger på dem (modtageren rebuilder dog hele træet ved pull).
        for ((uuid, group) in groupsToPush) {
            progress.onProgress(SyncProgressEvent.Pushing(++done, total))
            val mtime = group.times.modified
            val canonical = if (group.v == GroupSchemaVersion) group else group.copy(v = GroupSchemaVersion)
            val jsonBytes = CanonicalJson.encodeToString(Group.serializer(), canonical).toByteArray()
            val blob = crypto.encryptGroup(jsonBytes)
            val resp = api.putGroup(databaseId, uuid, blob, mtime.toJavaInstant())
            state.syncedAt[uuid] = mtime
            if (resp.seq > maxSeq) maxSeq = resp.seq
            pushedGroups++
        }

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
        return PushResult(pushedEntries, pushedGroups, pushedDeletions)
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
    /** Grupper dekrypteret og applied lokalt (v4 group-sync). */
    val pulledGroups: Int = 0,
    /** Tombstones modtaget og applied lokalt. */
    val pulledDeletions: Int,
    /** Entries hvor server var ældre end lokal — behold lokal, push den. */
    val pulledLocalKept: Int,
    /** Entries pushet til serveren i denne cyklus. */
    val pushedEntries: Int,
    /** Grupper pushet til serveren i denne cyklus (v4 group-sync). */
    val pushedGroups: Int = 0,
    /** Tombstones pushet til serveren i denne cyklus. */
    val pushedDeletions: Int,
    /** Den nye værdi af LocalState.lastSeq. */
    val newLastSeq: Long,
)
