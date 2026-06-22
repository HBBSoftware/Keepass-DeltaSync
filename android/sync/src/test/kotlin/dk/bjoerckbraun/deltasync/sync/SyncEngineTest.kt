// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import dk.bjoerckbraun.deltasync.api.ApiClient
import dk.bjoerckbraun.deltasync.canonical.CanonicalJson
import dk.bjoerckbraun.deltasync.canonical.Entry
import dk.bjoerckbraun.deltasync.canonical.EntryString
import dk.bjoerckbraun.deltasync.canonical.SchemaVersion
import dk.bjoerckbraun.deltasync.canonical.Times
import kotlinx.datetime.Instant
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import java.util.Base64
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

class SyncEngineTest {

    private lateinit var server: MockWebServer
    private lateinit var api: ApiClient
    private lateinit var crypto: FakeCryptoSession
    private lateinit var engine: SyncEngine

    private val dbId = "db-1234"

    @BeforeEach
    fun setup() {
        server = MockWebServer()
        server.start()
        api = ApiClient(
            baseUrl = server.url("/").toString().trimEnd('/'),
            deviceToken = "test-token",
        )
        crypto = FakeCryptoSession()
        engine = SyncEngine(api, crypto, dbId)
    }

    @AfterEach
    fun teardown() {
        server.shutdown()
    }

    @Test
    fun `empty server + empty local is a no-op`() {
        server.enqueue(jsonResponse("""{"current_seq":0,"entries":[]}"""))
        val state = LocalState()
        val result = engine.sync(state)
        assertEquals(0, result.pulledEntries)
        assertEquals(0, result.pushedEntries)
        assertEquals(0, result.newLastSeq)
        assertEquals(0L, state.lastSeq)
    }

    @Test
    fun `server entry is pulled into local state`() {
        val serverEntry = sampleEntry(uuid = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", title = "FromServer")
        val mtime = serverEntry.times.modified
        server.enqueue(jsonResponse("""
            {"current_seq":42,"entries":[
              {"uuid":"${serverEntry.uuid}","blob":"${encryptToBase64(serverEntry)}",
               "modified_at":"${mtime}","deleted":false,"seq":42,"available_versions":1}
            ]}
        """.trimIndent()))

        val state = LocalState()
        val result = engine.sync(state)

        assertEquals(1, result.pulledEntries)
        assertEquals(0, result.pushedEntries)
        assertEquals(42L, state.lastSeq)
        assertEquals("FromServer", state.entries[serverEntry.uuid]?.strings?.get("Title")?.v)
    }

    @Test
    fun `local-only entry is pushed`() {
        server.enqueue(jsonResponse("""{"current_seq":0,"entries":[]}"""))
        // PUT-respons.
        server.enqueue(jsonResponse("""
            {"entry":{"uuid":"new-uuid","modified_at":"2026-05-29T10:00:00Z",
                     "deleted":false,"seq":7,"created_at":"2026-05-29T10:00:00Z"}}
        """.trimIndent()))

        val local = sampleEntry(uuid = "00000000-0000-0000-0000-000000000001", title = "FromLocal")
        val state = LocalState(entries = mutableMapOf(local.uuid to local))

        val result = engine.sync(state)

        assertEquals(0, result.pulledEntries)
        assertEquals(1, result.pushedEntries)
        assertEquals(7L, state.lastSeq)
        assertEquals(local.times.modified, state.syncedAt[local.uuid])

        // Verificer at PUT-body indeholder en blob hvis "krypterede" payload
        // er en gyldig canonical Entry med Title=FromLocal.
        server.takeRequest() // GET /changes
        val putReq = server.takeRequest()
        assertEquals("PUT", putReq.method)
        val putBody = Json.parseToJsonElement(putReq.body.readUtf8()).jsonObject
        val blob = putBody["blob"]!!.jsonPrimitive.content
        val decrypted = crypto.decryptEntry(Base64.getDecoder().decode(blob))
        val parsed = CanonicalJson.decodeFromString(Entry.serializer(), String(decrypted))
        assertEquals("FromLocal", parsed.strings["Title"]?.v)
    }

    @Test
    fun `synced entry is not re-pushed`() {
        // Første kald: tom server + en lokal entry → push.
        server.enqueue(jsonResponse("""{"current_seq":0,"entries":[]}"""))
        server.enqueue(jsonResponse("""
            {"entry":{"uuid":"u1","modified_at":"2026-05-29T10:00:00Z",
                     "deleted":false,"seq":1,"created_at":"2026-05-29T10:00:00Z"}}
        """.trimIndent()))
        val entry = sampleEntry(uuid = "00000000-0000-0000-0000-000000000001", title = "OnlyOnce")
        val state = LocalState(entries = mutableMapOf(entry.uuid to entry))

        engine.sync(state)

        // Andet kald: tom server (intet nyt) + samme lokal entry → INGEN push.
        server.enqueue(jsonResponse("""{"current_seq":1,"entries":[]}"""))
        val result = engine.sync(state)

        assertEquals(0, result.pushedEntries)
    }

    @Test
    fun `server-newer entry replaces local when mtime is larger`() {
        val uuid = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
        val localOlder = sampleEntry(uuid = uuid, title = "OldLocal",
            modified = Instant.parse("2026-05-01T10:00:00Z"))
        val serverNewer = sampleEntry(uuid = uuid, title = "NewServer",
            modified = Instant.parse("2026-05-29T10:00:00Z"))

        server.enqueue(jsonResponse("""
            {"current_seq":5,"entries":[
              {"uuid":"$uuid","blob":"${encryptToBase64(serverNewer)}",
               "modified_at":"${serverNewer.times.modified}","deleted":false,"seq":5,
               "available_versions":1}
            ]}
        """.trimIndent()))

        val state = LocalState(
            entries = mutableMapOf(uuid to localOlder),
            syncedAt = mutableMapOf(uuid to localOlder.times.modified),
        )

        val result = engine.sync(state)

        assertEquals(1, result.pulledEntries)
        assertEquals(0, result.pushedEntries)
        assertEquals("NewServer", state.entries[uuid]?.strings?.get("Title")?.v)
    }

    @Test
    fun `local-newer entry wins and is pushed instead of replaced`() {
        val uuid = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
        val serverOlder = sampleEntry(uuid = uuid, title = "OldServer",
            modified = Instant.parse("2026-05-01T10:00:00Z"))
        val localNewer = sampleEntry(uuid = uuid, title = "NewLocal",
            modified = Instant.parse("2026-05-29T10:00:00Z"))

        server.enqueue(jsonResponse("""
            {"current_seq":3,"entries":[
              {"uuid":"$uuid","blob":"${encryptToBase64(serverOlder)}",
               "modified_at":"${serverOlder.times.modified}","deleted":false,"seq":3,
               "available_versions":1}
            ]}
        """.trimIndent()))
        server.enqueue(jsonResponse("""
            {"entry":{"uuid":"$uuid","modified_at":"${localNewer.times.modified}",
                     "deleted":false,"seq":4,"created_at":"2026-05-29T10:00:00Z"}}
        """.trimIndent()))

        val state = LocalState(entries = mutableMapOf(uuid to localNewer))

        val result = engine.sync(state)

        assertEquals(0, result.pulledEntries)
        assertEquals(1, result.pulledLocalKept) // server-version droppet
        assertEquals(1, result.pushedEntries)
        assertEquals("NewLocal", state.entries[uuid]?.strings?.get("Title")?.v)
        assertEquals(4L, state.lastSeq)
    }

    @Test
    fun `server tombstone removes local entry when local is not newer`() {
        val uuid = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
        val local = sampleEntry(uuid = uuid, title = "ToBeDeleted",
            modified = Instant.parse("2026-05-01T10:00:00Z"))
        val deletionAt = "2026-05-29T10:00:00Z"

        server.enqueue(jsonResponse("""
            {"current_seq":5,"entries":[
              {"uuid":"$uuid","blob":"","modified_at":"$deletionAt",
               "deleted":true,"seq":5,"available_versions":1}
            ]}
        """.trimIndent()))

        val state = LocalState(
            entries = mutableMapOf(uuid to local),
            syncedAt = mutableMapOf(uuid to local.times.modified),
        )

        val result = engine.sync(state)

        assertEquals(0, result.pulledEntries)
        assertEquals(1, result.pulledDeletions)
        assertTrue(uuid !in state.entries)
        assertEquals(Instant.parse(deletionAt), state.tombstones[uuid])
    }

    @Test
    fun `local deletion is pushed as tombstone`() {
        server.enqueue(jsonResponse("""{"current_seq":0,"entries":[]}"""))
        server.enqueue(jsonResponse("""
            {"entry":{"uuid":"u1","modified_at":"2026-05-29T10:00:00Z",
                     "deleted":true,"seq":9,"created_at":"2026-05-29T10:00:00Z"}}
        """.trimIndent()))

        val uuid = "00000000-0000-0000-0000-000000000001"
        val state = LocalState(
            tombstones = mutableMapOf(uuid to Instant.parse("2026-05-29T10:00:00Z")),
        )

        val result = engine.sync(state)

        assertEquals(1, result.pushedDeletions)
        assertEquals(9L, state.lastSeq)

        server.takeRequest() // GET
        val delReq = server.takeRequest()
        assertEquals("DELETE", delReq.method)
        assertTrue(delReq.path!!.endsWith("/entries/$uuid"))
    }

    @Test
    fun `lastSeq advances to max of changes_current_seq and put seq`() {
        // /changes returnerer current_seq=10 men ingen entries. Vores push
        // returnerer seq=15. lastSeq skal være 15 efter sync.
        server.enqueue(jsonResponse("""{"current_seq":10,"entries":[]}"""))
        server.enqueue(jsonResponse("""
            {"entry":{"uuid":"u","modified_at":"2026-05-29T10:00:00Z",
                     "deleted":false,"seq":15,"created_at":"2026-05-29T10:00:00Z"}}
        """.trimIndent()))

        val entry = sampleEntry(uuid = "00000000-0000-0000-0000-000000000001")
        val state = LocalState(entries = mutableMapOf(entry.uuid to entry))

        engine.sync(state)
        assertEquals(15L, state.lastSeq)
    }

    @Test
    fun `api error throws ApiException`() {
        server.enqueue(
            MockResponse()
                .setResponseCode(401)
                .setBody("""{"error":{"code":"unauthorized","message":"bad token"}}""")
                .addHeader("Content-Type", "application/json")
        )
        val state = LocalState()
        val ex = runCatching { engine.sync(state) }.exceptionOrNull()
        assertNotNull(ex)
        assertTrue(ex.message!!.contains("unauthorized"), "actual: ${ex.message}")
    }

    // --- Helpers ---

    /**
     * Fake CryptoSession der bare identity-passer bytes — perfekt til at
     * teste sync-state-machine'en uden krypto-kompleksitet. Den ægte impl
     * (mobile.Session via gomobile bind) tester vi separat på Go-siden.
     */
    private class FakeCryptoSession : CryptoSession {
        private var closed = false
        override fun encryptEntry(entryJson: ByteArray): ByteArray {
            check(!closed)
            return entryJson
        }
        override fun decryptEntry(blob: ByteArray): ByteArray {
            check(!closed)
            return blob
        }
        override fun encryptGroup(groupJson: ByteArray): ByteArray {
            check(!closed)
            return groupJson
        }
        override fun decryptGroup(blob: ByteArray): ByteArray {
            check(!closed)
            return blob
        }
        override fun close() { closed = true }
    }

    private fun sampleEntry(
        uuid: String,
        title: String = "Sample",
        modified: Instant = Instant.parse("2026-05-29T10:00:00Z"),
    ): Entry = Entry(
        v = SchemaVersion,
        uuid = uuid,
        times = Times(
            created = modified,
            modified = modified,
            accessed = modified,
            locationChanged = modified,
        ),
        strings = mapOf("Title" to EntryString(v = title)),
    )

    private fun encryptToBase64(entry: Entry): String {
        val json = CanonicalJson.encodeToString(Entry.serializer(), entry).toByteArray()
        val blob = crypto.encryptEntry(json)
        return Base64.getEncoder().encodeToString(blob)
    }

    private fun jsonResponse(body: String): MockResponse =
        MockResponse()
            .setBody(body)
            .addHeader("Content-Type", "application/json")
}
