// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.api

import dk.bjoerckbraun.deltasync.sync.InMemoryTokenStore
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
import kotlin.test.assertNull
import kotlin.test.assertTrue

class EnrollmentClientTest {

    private lateinit var server: MockWebServer
    private lateinit var client: EnrollmentClient

    @BeforeEach
    fun setup() {
        server = MockWebServer()
        server.start()
        client = EnrollmentClient(baseUrl = server.url("/").toString().trimEnd('/'))
    }

    @AfterEach
    fun teardown() {
        server.shutdown()
    }

    @Test
    fun `enroll posts public key and returns device-token`() {
        server.enqueue(
            MockResponse()
                .setResponseCode(201)
                .setBody("""
                    {"device":{"id":"dev-abc","name":"Pixel 8","enrolled_at":"2026-05-29T10:00:00Z"},
                     "token":"perma-token-xyz"}
                """.trimIndent())
                .addHeader("Content-Type", "application/json")
        )

        val publicKey = ByteArray(32) { 0x42.toByte() }
        val result = client.enroll(
            enrollmentToken = "one-time-enroll",
            deviceName = "Pixel 8",
            devicePublicKey = publicKey,
        )

        assertEquals("dev-abc", result.deviceId)
        assertEquals("Pixel 8", result.deviceName)
        assertEquals("perma-token-xyz", result.deviceToken)

        val req = server.takeRequest()
        assertEquals("POST", req.method)
        assertEquals("/api/v1/devices/enroll", req.path)
        assertEquals("Bearer one-time-enroll", req.getHeader("Authorization"))

        val body = Json.parseToJsonElement(req.body.readUtf8()).jsonObject
        assertEquals("Pixel 8", body["device_name"]?.jsonPrimitive?.content)
        assertEquals(
            Base64.getEncoder().encodeToString(publicKey),
            body["public_key"]?.jsonPrimitive?.content,
        )
    }

    @Test
    fun `enroll omits device_name when blank`() {
        server.enqueue(
            MockResponse()
                .setResponseCode(201)
                .setBody("""
                    {"device":{"id":"d","name":"d","enrolled_at":"x"},"token":"t"}
                """.trimIndent())
                .addHeader("Content-Type", "application/json")
        )

        client.enroll(
            enrollmentToken = "tok",
            deviceName = "",
            devicePublicKey = ByteArray(32),
        )

        val body = Json.parseToJsonElement(server.takeRequest().body.readUtf8()).jsonObject
        assertNull(body["device_name"], "device_name should be omitted when blank")
    }

    @Test
    fun `enroll throws ApiException on invalid token`() {
        server.enqueue(
            MockResponse()
                .setResponseCode(401)
                .setBody("""{"error":{"code":"invalid_token","message":"enrollment token expired"}}""")
                .addHeader("Content-Type", "application/json")
        )

        val ex = runCatching {
            client.enroll(
                enrollmentToken = "expired",
                deviceName = null,
                devicePublicKey = ByteArray(32),
            )
        }.exceptionOrNull()

        assertNotNull(ex)
        val api = ex as ApiException
        assertEquals(401, api.statusCode)
        assertEquals("invalid_token", api.code)
        assertTrue(api.detail.contains("expired"), "detail: ${api.detail}")
    }

    @Test
    fun `TokenStore in-memory round-trip persists credentials`() {
        val store = InMemoryTokenStore()
        assertNull(store.load())

        val priv = ByteArray(32) { 0xaa.toByte() }
        store.save(
            serverUrl = "https://example.com/",
            deviceId = "dev-1",
            deviceToken = "tok",
            devicePrivateKey = priv,
        )

        val loaded = store.load()
        assertNotNull(loaded)
        assertEquals("https://example.com", loaded.serverUrl) // trailing-slash trimmed
        assertEquals("dev-1", loaded.deviceId)
        assertEquals("tok", loaded.deviceToken)
        assertTrue(loaded.devicePrivateKey.contentEquals(priv))
    }

    @Test
    fun `TokenStore clear zeros private key and removes credentials`() {
        val store = InMemoryTokenStore()
        val priv = ByteArray(32) { 0xff.toByte() }
        store.save("u", "d", "t", priv)

        store.clear()

        assertNull(store.load())
        // Caller's array bør ikke være mutated — vi copy'er på save.
        assertTrue(
            priv.all { it == 0xff.toByte() },
            "caller's array should NOT be mutated (we copy on save)",
        )
    }
}
