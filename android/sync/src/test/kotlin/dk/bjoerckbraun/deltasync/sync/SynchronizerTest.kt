// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import app.keemobile.kotpass.cryptography.EncryptedValue
import app.keemobile.kotpass.database.Credentials
import app.keemobile.kotpass.database.KeePassDatabase
import app.keemobile.kotpass.database.decode
import app.keemobile.kotpass.database.encode
import app.keemobile.kotpass.database.findEntries
import app.keemobile.kotpass.database.modifiers.modifyParentGroup
import app.keemobile.kotpass.models.EntryFields
import app.keemobile.kotpass.models.EntryValue
import app.keemobile.kotpass.models.Meta
import app.keemobile.kotpass.models.TimeData
import dk.bjoerckbraun.deltasync.api.ApiClient
import dk.bjoerckbraun.deltasync.canonical.CanonicalJson
import dk.bjoerckbraun.deltasync.canonical.Entry
import dk.bjoerckbraun.deltasync.canonical.EntryString
import dk.bjoerckbraun.deltasync.canonical.SchemaVersion
import dk.bjoerckbraun.deltasync.canonical.Times
import kotlinx.datetime.Instant
import kotlinx.datetime.toJavaInstant
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Files
import java.nio.file.Path
import java.util.Base64
import java.util.UUID
import kotlin.test.assertEquals
import kotlin.test.assertTrue
import app.keemobile.kotpass.models.Entry as KotpassEntry

/**
 * End-to-end-test af hele synkroniserings-pipelinen mod en MockWebServer:
 * load fra disk → adapter → sync-engine → adapter → encode til disk.
 * Verificerer at lokale .kdbx-filer korrekt mirrors med server-state
 * og at sync-tracking persisteres mellem kald.
 */
class SynchronizerTest {

    @TempDir
    lateinit var tempDir: Path

    private lateinit var server: MockWebServer
    private lateinit var api: ApiClient
    private lateinit var persistence: InMemorySyncStatePersistence
    private val crypto = IdentityCryptoSession()

    private val dbId = "test-db"
    private val passphrase = "test-passphrase"
    private val credentials get() = Credentials.from(EncryptedValue.fromString(passphrase))

    @BeforeEach
    fun setup() {
        server = MockWebServer()
        server.start()
        api = ApiClient(
            baseUrl = server.url("/").toString().trimEnd('/'),
            deviceToken = "test-token",
        )
        persistence = InMemorySyncStatePersistence()
    }

    @AfterEach
    fun teardown() {
        server.shutdown()
    }

    @Test
    fun `pulls a new entry from server and writes it back to kdbx`() {
        // Brug en tom kdbx for at isolere pull-stien; en lokal entry ville
        // ellers blive pushet og kræve en ekstra MockResponse.
        val kdbxPath = createEmptyKdbx()
        val serverEntry = sampleCanonicalEntry(
            uuid = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
            title = "FromServer",
        )

        server.enqueue(jsonResponse("""
            {"current_seq":42,"entries":[
              {"uuid":"${serverEntry.uuid}","blob":"${encryptToBase64(serverEntry)}",
               "modified_at":"${serverEntry.times.modified}","deleted":false,"seq":42,
               "available_versions":1}
            ]}
        """.trimIndent()))

        val sync = Synchronizer(PathKdbxFile(kdbxPath), credentials, api, crypto, persistence)
        val result = sync.sync(dbId)

        assertEquals(1, result.pulledEntries)
        assertEquals(42L, result.newLastSeq)

        // Verificer at .kdbx-filen nu indeholder server-entry'en.
        val titles = Files.newInputStream(kdbxPath).use { input ->
            val db = KeePassDatabase.decode(input, credentials)
            db.findEntries { true }.flatMap { (_, list) -> list }
                .mapNotNull { it.fields["Title"]?.content }
                .toSet()
        }
        assertTrue("FromServer" in titles, "actual titles: $titles")

        // Persistens skal være opdateret.
        val persisted = persistence.load(dbId)
        assertEquals(42L, persisted.lastSeq)
    }

    @Test
    fun `pushes a local entry that the server doesn't have yet`() {
        val kdbxPath = createKdbxWithOneLocalEntry()

        // Server returnerer ingen ændringer.
        server.enqueue(jsonResponse("""{"current_seq":0,"entries":[]}"""))
        // PUT-respons for vores lokale entry.
        server.enqueue(jsonResponse("""
            {"entry":{"uuid":"local-uuid","modified_at":"2026-05-29T10:00:00Z",
                     "deleted":false,"seq":5,"created_at":"2026-05-29T10:00:00Z"}}
        """.trimIndent()))

        val sync = Synchronizer(PathKdbxFile(kdbxPath), credentials, api, crypto, persistence)
        val result = sync.sync(dbId)

        assertEquals(0, result.pulledEntries)
        assertEquals(1, result.pushedEntries)
        assertEquals(5L, persistence.load(dbId).lastSeq)

        // Verificer at PUT request indeholdt vores lokale entry.
        server.takeRequest() // GET /changes
        val putReq = server.takeRequest()
        assertEquals("PUT", putReq.method)
    }

    @Test
    fun `subsequent sync with no changes is a no-op`() {
        val kdbxPath = createKdbxWithOneLocalEntry()

        // Første sync: server tom + 1 lokal entry → push.
        server.enqueue(jsonResponse("""{"current_seq":0,"entries":[]}"""))
        server.enqueue(jsonResponse("""
            {"entry":{"uuid":"local-uuid","modified_at":"2026-05-29T10:00:00Z",
                     "deleted":false,"seq":1,"created_at":"2026-05-29T10:00:00Z"}}
        """.trimIndent()))
        Synchronizer(PathKdbxFile(kdbxPath), credentials, api, crypto, persistence).sync(dbId)

        // Anden sync: server siger "ingen nye changes" og vi har intet at pushe.
        server.enqueue(jsonResponse("""{"current_seq":1,"entries":[]}"""))
        val second = Synchronizer(PathKdbxFile(kdbxPath), credentials, api, crypto, persistence).sync(dbId)

        assertEquals(0, second.pulledEntries)
        assertEquals(0, second.pushedEntries)
    }

    @Test
    fun `kdbx file is not rewritten when only pushes happen`() {
        // Optimering: hvis vi kun pushede (intet pullet) skal vi ikke
        // skrive .kdbx tilbage — filen er allerede den sandhed serveren
        // nu også har. Vi verificerer dette ved at tjekke mtime.
        val kdbxPath = createKdbxWithOneLocalEntry()
        val mtimeBefore = Files.getLastModifiedTime(kdbxPath)

        server.enqueue(jsonResponse("""{"current_seq":0,"entries":[]}"""))
        server.enqueue(jsonResponse("""
            {"entry":{"uuid":"local-uuid","modified_at":"2026-05-29T10:00:00Z",
                     "deleted":false,"seq":1,"created_at":"2026-05-29T10:00:00Z"}}
        """.trimIndent()))

        // Lille søvn så vi kan detektere ny mtime hvis filen blev rørt.
        Thread.sleep(50)

        Synchronizer(PathKdbxFile(kdbxPath), credentials, api, crypto, persistence).sync(dbId)

        val mtimeAfter = Files.getLastModifiedTime(kdbxPath)
        assertEquals(mtimeBefore, mtimeAfter, "kdbx file should not have been rewritten on push-only sync")
    }

    @Test
    fun `pulled deletion removes entry from kdbx`() {
        val localUuid = UUID.fromString("99999999-9999-9999-9999-999999999999")
        val kdbxPath = tempDir.resolve("delete-test.kdbx")
        val db = KeePassDatabase.Ver4x.create(
            rootName = "Root",
            meta = Meta(generator = "test"),
            credentials = credentials,
        ).modifyParentGroup {
            copy(entries = entries + kotpassEntry(localUuid, "ToBeDeleted"))
        }
        Files.newOutputStream(kdbxPath).use { db.encode(it) }

        // Server siger: slet den entry.
        server.enqueue(jsonResponse("""
            {"current_seq":10,"entries":[
              {"uuid":"${localUuid}","blob":"","modified_at":"2026-05-29T10:00:00Z",
               "deleted":true,"seq":10,"available_versions":1}
            ]}
        """.trimIndent()))

        val sync = Synchronizer(PathKdbxFile(kdbxPath), credentials, api, crypto, persistence)
        val result = sync.sync(dbId)

        assertEquals(1, result.pulledDeletions)

        val titles = Files.newInputStream(kdbxPath).use { input ->
            KeePassDatabase.decode(input, credentials).findEntries { true }
                .flatMap { (_, list) -> list }
                .mapNotNull { it.fields["Title"]?.content }
        }
        assertEquals(emptyList(), titles, "deleted entry should be gone")
    }

    // --- Helpers ---

    private fun createEmptyKdbx(): Path {
        val path = tempDir.resolve("empty.kdbx")
        val db = KeePassDatabase.Ver4x.create(
            rootName = "Root",
            meta = Meta(generator = "test"),
            credentials = credentials,
        )
        Files.newOutputStream(path).use { db.encode(it) }
        return path
    }

    private fun createKdbxWithOneLocalEntry(): Path {
        val path = tempDir.resolve("test.kdbx")
        val db = KeePassDatabase.Ver4x.create(
            rootName = "Root",
            meta = Meta(generator = "test"),
            credentials = credentials,
        ).modifyParentGroup {
            copy(entries = entries + kotpassEntry(
                uuid = UUID.fromString("00000000-0000-0000-0000-000000000001"),
                title = "Local",
            ))
        }
        Files.newOutputStream(path).use { db.encode(it) }
        return path
    }

    private fun kotpassEntry(uuid: UUID, title: String): KotpassEntry {
        val now = Instant.parse("2026-05-29T10:00:00Z").toJavaInstant()
        return KotpassEntry(
            uuid = uuid,
            fields = EntryFields(mapOf("Title" to EntryValue.Plain(title))),
            times = TimeData(
                creationTime = now,
                lastModificationTime = now,
                lastAccessTime = now,
                expiryTime = null,
                locationChanged = now,
                expires = false,
                usageCount = 0,
            ),
        )
    }

    private fun sampleCanonicalEntry(uuid: String, title: String) = Entry(
        v = SchemaVersion,
        uuid = uuid,
        times = Times(
            created = Instant.parse("2026-05-29T10:00:00Z"),
            modified = Instant.parse("2026-05-29T10:00:00Z"),
            accessed = Instant.parse("2026-05-29T10:00:00Z"),
            locationChanged = Instant.parse("2026-05-29T10:00:00Z"),
        ),
        strings = mapOf("Title" to EntryString(v = title)),
    )

    private fun encryptToBase64(entry: Entry): String {
        val json = CanonicalJson.encodeToString(Entry.serializer(), entry).toByteArray()
        val blob = crypto.encryptEntry(json)
        return Base64.getEncoder().encodeToString(blob)
    }

    private fun jsonResponse(body: String) = MockResponse()
        .setBody(body)
        .addHeader("Content-Type", "application/json")

    /** Identity-pass crypto for end-to-end-tests; ikke ægte krypto. */
    private class IdentityCryptoSession : CryptoSession {
        override fun encryptEntry(entryJson: ByteArray): ByteArray = entryJson
        override fun decryptEntry(blob: ByteArray): ByteArray = blob
        override fun close() {}
    }
}
