// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.api

import kotlinx.serialization.json.Json
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import java.io.IOException
import java.time.Instant
import java.time.format.DateTimeFormatter

/**
 * Tynd HTTP-klient mod Delta-Sync-serveren. Spejler den Go-side `api.Client`
 * desktop-klienten bruger — endpoint-veje, JSON-payloads, og auth-header
 * skal være eksakt ens for at klienterne kan dele en server.
 *
 * Klienten er bevidst minimal: den dækker det sync-loopet bruger
 * (changes, putEntry, deleteEntry, listDatabases). Enrollment, share-
 * operationer og restore tilføjes når Android-UI'en har brug for dem.
 */
class ApiClient(
    private val baseUrl: String,
    private val deviceToken: String,
    private val httpClient: OkHttpClient = defaultHttp(),
) {
    private val json = Json {
        ignoreUnknownKeys = true
        encodeDefaults = true
    }

    /** GET `/api/v1/databases`. */
    @Throws(ApiException::class, IOException::class)
    fun listDatabases(): List<Database> {
        val url = "${baseUrl.trimEnd('/')}/api/v1/databases".toHttpUrl()
        val req = Request.Builder()
            .url(url)
            .get()
            .authed()
            .build()
        return httpClient.newCall(req).execute().use { resp ->
            ensureSuccess(resp)
            json.decodeFromString(DatabaseListResponse.serializer(), resp.bodyString()).databases
        }
    }

    /** GET `/api/v1/databases/{id}/changes?since=N`. */
    @Throws(ApiException::class, IOException::class)
    fun getChanges(databaseId: String, since: Long, includeGroups: Boolean = false): ChangesResponse {
        val base = "${baseUrl.trimEnd('/')}/api/v1/databases/$databaseId/changes?since=$since"
        val url = (if (includeGroups) "$base&include=groups" else base).toHttpUrl()
        val req = Request.Builder().url(url).get().authed().build()
        return httpClient.newCall(req).execute().use { resp ->
            ensureSuccess(resp)
            json.decodeFromString(ChangesResponse.serializer(), resp.bodyString())
        }
    }

    /** PUT `/api/v1/databases/{id}/entries/{uuid}`. */
    @Throws(ApiException::class, IOException::class)
    fun putEntry(databaseId: String, entryUuid: String, blob: ByteArray, modifiedAt: Instant): EntryPutResponse {
        val body = json.encodeToString(
            PutEntryRequest.serializer(),
            PutEntryRequest(
                blob = blob.toBase64(),
                modifiedAt = formatIso(modifiedAt),
            )
        )
        val url = "${baseUrl.trimEnd('/')}/api/v1/databases/$databaseId/entries/$entryUuid".toHttpUrl()
        val req = Request.Builder()
            .url(url)
            .put(body.toRequestBody(JSON_MEDIA))
            .authed()
            .build()
        return httpClient.newCall(req).execute().use { resp ->
            ensureSuccess(resp)
            json.decodeFromString(EntryPutEnvelope.serializer(), resp.bodyString()).entry
        }
    }

    /** DELETE `/api/v1/databases/{id}/entries/{uuid}` — tombstone. */
    @Throws(ApiException::class, IOException::class)
    fun deleteEntry(databaseId: String, entryUuid: String, modifiedAt: Instant): EntryPutResponse {
        val body = json.encodeToString(
            DeleteEntryRequest.serializer(),
            DeleteEntryRequest(modifiedAt = formatIso(modifiedAt)),
        )
        val url = "${baseUrl.trimEnd('/')}/api/v1/databases/$databaseId/entries/$entryUuid".toHttpUrl()
        val req = Request.Builder()
            .url(url)
            .delete(body.toRequestBody(JSON_MEDIA))
            .authed()
            .build()
        return httpClient.newCall(req).execute().use { resp ->
            ensureSuccess(resp)
            json.decodeFromString(EntryPutEnvelope.serializer(), resp.bodyString()).entry
        }
    }

    /** PUT `/api/v1/databases/{id}/groups/{uuid}` (v4 group-sync, object_kind=2). */
    @Throws(ApiException::class, IOException::class)
    fun putGroup(databaseId: String, groupUuid: String, blob: ByteArray, modifiedAt: Instant): EntryPutResponse {
        val body = json.encodeToString(
            PutEntryRequest.serializer(),
            PutEntryRequest(blob = blob.toBase64(), modifiedAt = formatIso(modifiedAt)),
        )
        val url = "${baseUrl.trimEnd('/')}/api/v1/databases/$databaseId/groups/$groupUuid".toHttpUrl()
        val req = Request.Builder().url(url).put(body.toRequestBody(JSON_MEDIA)).authed().build()
        return httpClient.newCall(req).execute().use { resp ->
            ensureSuccess(resp)
            json.decodeFromString(GroupPutEnvelope.serializer(), resp.bodyString()).group
        }
    }

    /** DELETE `/api/v1/databases/{id}/groups/{uuid}` — gruppe-tombstone. */
    @Throws(ApiException::class, IOException::class)
    fun deleteGroup(databaseId: String, groupUuid: String, modifiedAt: Instant): EntryPutResponse {
        val body = json.encodeToString(
            DeleteEntryRequest.serializer(),
            DeleteEntryRequest(modifiedAt = formatIso(modifiedAt)),
        )
        val url = "${baseUrl.trimEnd('/')}/api/v1/databases/$databaseId/groups/$groupUuid".toHttpUrl()
        val req = Request.Builder().url(url).delete(body.toRequestBody(JSON_MEDIA)).authed().build()
        return httpClient.newCall(req).execute().use { resp ->
            ensureSuccess(resp)
            json.decodeFromString(GroupPutEnvelope.serializer(), resp.bodyString()).group
        }
    }

    /** GET `/api/v1/users/lookup?username=X`. */
    @Throws(ApiException::class, IOException::class)
    fun lookupUser(username: String): UserLookup {
        val url = "${baseUrl.trimEnd('/')}/api/v1/users/lookup".toHttpUrl()
            .newBuilder()
            .addQueryParameter("username", username)
            .build()
        val req = Request.Builder().url(url).get().authed().build()
        return httpClient.newCall(req).execute().use { resp ->
            ensureSuccess(resp)
            json.decodeFromString(UserLookup.serializer(), resp.bodyString())
        }
    }

    /** GET `/api/v1/databases/{id}/shares` — owner-only på serveren. */
    @Throws(ApiException::class, IOException::class)
    fun listShares(databaseId: String): List<ShareMember> {
        val url = "${baseUrl.trimEnd('/')}/api/v1/databases/$databaseId/shares".toHttpUrl()
        val req = Request.Builder().url(url).get().authed().build()
        return httpClient.newCall(req).execute().use { resp ->
            ensureSuccess(resp)
            json.decodeFromString(ShareListResponse.serializer(), resp.bodyString()).members
        }
    }

    /** POST `/api/v1/databases/{id}/shares` — tilføj/rotér en members wrapped key. */
    @Throws(ApiException::class, IOException::class)
    fun shareDatabase(databaseId: String, targetUserId: String, wrappedKey: ByteArray) {
        val body = json.encodeToString(
            ShareRequest.serializer(),
            ShareRequest(userId = targetUserId, wrappedMasterKey = wrappedKey.toBase64()),
        )
        val url = "${baseUrl.trimEnd('/')}/api/v1/databases/$databaseId/shares".toHttpUrl()
        val req = Request.Builder().url(url).post(body.toRequestBody(JSON_MEDIA)).authed().build()
        httpClient.newCall(req).execute().use { resp -> ensureSuccess(resp) }
    }

    /** DELETE `/api/v1/databases/{id}/shares/{user_id}`. */
    @Throws(ApiException::class, IOException::class)
    fun unshareDatabase(databaseId: String, targetUserId: String) {
        val url = "${baseUrl.trimEnd('/')}/api/v1/databases/$databaseId/shares/$targetUserId".toHttpUrl()
        val req = Request.Builder().url(url).delete().authed().build()
        httpClient.newCall(req).execute().use { resp -> ensureSuccess(resp) }
    }

    private fun Request.Builder.authed(): Request.Builder = this
        .header("Authorization", "Bearer $deviceToken")
        .header("User-Agent", USER_AGENT)
        .header("Accept", "application/json")

    private fun ensureSuccess(resp: Response) {
        if (resp.isSuccessful) return
        val raw = resp.bodyString()
        val parsed = runCatching {
            json.decodeFromString(ErrorEnvelope.serializer(), raw).error
        }.getOrNull()
        throw ApiException(
            statusCode = resp.code,
            code = parsed?.code ?: "",
            detail = parsed?.message ?: raw.take(200),
        )
    }

    private fun ByteArray.toBase64(): String =
        java.util.Base64.getEncoder().encodeToString(this)

    private fun formatIso(t: Instant): String =
        DateTimeFormatter.ISO_INSTANT.format(t.atZone(java.time.ZoneOffset.UTC))
            // Server forventer "2006-01-02T15:04:05Z" uden sub-sekunder. ISO_INSTANT
            // dropper sub-sekunder hvis de er nul, hvilket matcher når vi
            // round-tripper canonical-times (sekund-præcision).
            .let { if (it.endsWith("Z")) it else "${it}Z" }

    private fun Response.bodyString(): String = body?.string().orEmpty()

    companion object {
        private const val USER_AGENT = "keepass-deltasync-android/0.1.0"
        private val JSON_MEDIA = "application/json".toMediaType()

        fun defaultHttp(): OkHttpClient = OkHttpClient.Builder()
            .connectTimeout(15, java.util.concurrent.TimeUnit.SECONDS)
            .readTimeout(30, java.util.concurrent.TimeUnit.SECONDS)
            .build()
    }
}

/** Typed undtagelse for ikke-2xx HTTP-responses. */
class ApiException(
    val statusCode: Int,
    val code: String,
    /** Server-leveret fejl-besked uden status/code-prefix. Den fulde
     *  formatterede besked lever på [Throwable.message]. */
    val detail: String,
) : RuntimeException(
    "$statusCode${if (code.isNotEmpty()) " ($code)" else ""}: $detail"
)
