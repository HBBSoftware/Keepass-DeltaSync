// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.api

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import java.io.IOException
import java.util.Base64

/**
 * Klient for første-gangs enrollment-flowet: bytter en éngangs
 * enrollment-token (udleveret af serverens administrator) for et
 * permanent device-token + device-id. Er adskilt fra [ApiClient]
 * fordi den ikke har en device-token endnu og bruger en anden
 * authorization-header (enrollment-token i stedet for device-token).
 *
 * Flowet:
 *
 *   1. Generér X25519 device-keypair via
 *      [dk.bjoerckbraun.deltasync.sync.GomobileCryptoSession.generateDeviceKeypair].
 *   2. Kald [enroll] med enrollment-token, valgfrit device-name, og
 *      device public-key.
 *   3. Gem [EnrollmentResult.deviceToken] og device private-key i en
 *      sikker [dk.bjoerckbraun.deltasync.sync.TokenStore].
 */
class EnrollmentClient(
    private val baseUrl: String,
    private val httpClient: OkHttpClient = ApiClient.defaultHttp(),
) {
    private val json = Json {
        ignoreUnknownKeys = true
        encodeDefaults = true
        // device_name skal helt udelades hvis null — matcher Go's omitempty
        // og holder request-body'en minimal.
        explicitNulls = false
    }

    /**
     * POST `/api/v1/devices/enroll`. Returnerer device-info + det
     * permanente device-token når server'en accepterer enrollment-tokenet.
     * Smider [ApiException] for non-201-responser.
     */
    @Throws(ApiException::class, IOException::class)
    fun enroll(
        enrollmentToken: String,
        deviceName: String?,
        devicePublicKey: ByteArray,
    ): EnrollmentResult {
        val body = json.encodeToString(EnrollmentRequest.serializer(), EnrollmentRequest(
            deviceName = deviceName?.takeIf { it.isNotBlank() },
            publicKey = Base64.getEncoder().encodeToString(devicePublicKey),
        ))

        val url = "${baseUrl.trimEnd('/')}/api/v1/devices/enroll".toHttpUrl()
        val req = Request.Builder()
            .url(url)
            .post(body.toRequestBody(JSON_MEDIA))
            .header("Authorization", "Bearer $enrollmentToken")
            .header("Content-Type", "application/json")
            .header("Accept", "application/json")
            .header("User-Agent", USER_AGENT)
            .build()

        return httpClient.newCall(req).execute().use { resp ->
            ensureCreated(resp)
            val envelope = json.decodeFromString(EnrollmentEnvelope.serializer(), resp.body?.string().orEmpty())
            EnrollmentResult(
                deviceId = envelope.device.id,
                deviceName = envelope.device.name,
                enrolledAt = envelope.device.enrolledAt,
                deviceToken = envelope.token,
            )
        }
    }

    private fun ensureCreated(resp: Response) {
        if (resp.code == 201) return
        val raw = resp.body?.string().orEmpty()
        val parsed = runCatching {
            json.decodeFromString(ErrorEnvelope.serializer(), raw).error
        }.getOrNull()
        throw ApiException(
            statusCode = resp.code,
            code = parsed?.code ?: "",
            detail = parsed?.message ?: raw.take(200),
        )
    }

    companion object {
        private const val USER_AGENT = "keepass-deltasync-android/0.1.0"
        private val JSON_MEDIA = "application/json".toMediaType()
    }
}

/** Resultat af succesfuld enrollment. */
data class EnrollmentResult(
    val deviceId: String,
    val deviceName: String,
    val enrolledAt: String,
    /** Permanent device-token. Skal gemmes i [TokenStore]. */
    val deviceToken: String,
)

@Serializable
internal data class EnrollmentRequest(
    @SerialName("device_name") val deviceName: String? = null,
    @SerialName("public_key") val publicKey: String,
)

@Serializable
internal data class EnrollmentEnvelope(
    val device: EnrolledDevice,
    val token: String,
)

@Serializable
internal data class EnrolledDevice(
    val id: String,
    val name: String,
    @SerialName("enrolled_at") val enrolledAt: String,
)
