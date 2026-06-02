// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.api

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

/** Dækker v2 share-endpoints på [ApiClient]: lookup, list, share, unshare. */
class ApiClientShareTest {

    private lateinit var server: MockWebServer
    private lateinit var client: ApiClient

    private val dbId = "11111111-1111-1111-1111-111111111111"

    @BeforeEach
    fun setup() {
        server = MockWebServer()
        server.start()
        client = ApiClient(
            baseUrl = server.url("/").toString().trimEnd('/'),
            deviceToken = "dev-token",
        )
    }

    @AfterEach
    fun teardown() {
        server.shutdown()
    }

    @Test
    fun `lookupUser parses user and target device`() {
        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setBody("""
                    {"user":{"id":"u-bob","username":"bob","display_name":"Bob B"},
                     "target_device":{"id":"d-1","name":"Bob Pixel","public_key":"cHVia2V5","enrolled_at":"2026-06-01T10:00:00Z"}}
                """.trimIndent())
                .addHeader("Content-Type", "application/json")
        )

        val result = client.lookupUser("bob")

        assertEquals("u-bob", result.user.id)
        assertEquals("bob", result.user.username)
        assertEquals("cHVia2V5", result.targetDevice.publicKey)

        val req = server.takeRequest()
        assertEquals("GET", req.method)
        assertEquals("/api/v1/users/lookup?username=bob", req.path)
        assertEquals("Bearer dev-token", req.getHeader("Authorization"))
    }

    @Test
    fun `listShares parses members`() {
        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setBody("""
                    {"members":[
                      {"user_id":"u-alice","username":"alice","display_name":null,"role":"owner","added_at":"2026-05-01T00:00:00Z","added_by":null},
                      {"user_id":"u-bob","username":"bob","display_name":"Bob B","role":"member","added_at":"2026-06-01T00:00:00Z","added_by":"u-alice"}
                    ]}
                """.trimIndent())
                .addHeader("Content-Type", "application/json")
        )

        val members = client.listShares(dbId)

        assertEquals(2, members.size)
        assertEquals("owner", members[0].role)
        assertEquals("bob", members[1].username)
        assertEquals("/api/v1/databases/$dbId/shares", server.takeRequest().path)
    }

    @Test
    fun `shareDatabase posts user_id and base64 wrapped key`() {
        server.enqueue(MockResponse().setResponseCode(201).setBody("{}"))

        val wrapped = ByteArray(48) { 0x07.toByte() }
        client.shareDatabase(dbId, "u-bob", wrapped)

        val req = server.takeRequest()
        assertEquals("POST", req.method)
        assertEquals("/api/v1/databases/$dbId/shares", req.path)
        val body = Json.parseToJsonElement(req.body.readUtf8()).jsonObject
        assertEquals("u-bob", body["user_id"]?.jsonPrimitive?.content)
        assertEquals(
            Base64.getEncoder().encodeToString(wrapped),
            body["wrapped_master_key"]?.jsonPrimitive?.content,
        )
    }

    @Test
    fun `unshareDatabase deletes the member`() {
        server.enqueue(MockResponse().setResponseCode(204))

        client.unshareDatabase(dbId, "u-bob")

        val req = server.takeRequest()
        assertEquals("DELETE", req.method)
        assertEquals("/api/v1/databases/$dbId/shares/u-bob", req.path)
    }

    @Test
    fun `listShares surfaces 403 as ApiException for non-owner`() {
        server.enqueue(
            MockResponse()
                .setResponseCode(403)
                .setBody("""{"error":{"code":"forbidden","message":"only the owner can manage sharing"}}""")
                .addHeader("Content-Type", "application/json")
        )

        val ex = runCatching { client.listShares(dbId) }.exceptionOrNull()

        assertNotNull(ex)
        val api = ex as ApiException
        assertEquals(403, api.statusCode)
        assertTrue(api.detail.contains("owner"), "detail: ${api.detail}")
    }
}
