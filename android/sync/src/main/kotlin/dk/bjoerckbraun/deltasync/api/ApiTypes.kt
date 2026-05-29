// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.api

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * JSON-skemaer der spejler det HTTP-API serveren eksponerer
 * (`client/internal/api/client.go` på Go-siden). Hold strukturerne i
 * eksakt felt-navne-overensstemmelse med Go-versionerne; serveren accepterer
 * ikke camelCase.
 */

/** Én entry-version som returneret af `/changes` og POST/PUT-respons. */
@Serializable
data class EntryChange(
    val uuid: String,
    val blob: String,
    @SerialName("modified_at") val modifiedAt: String,
    val deleted: Boolean = false,
    val seq: Long,
    @SerialName("available_versions") val availableVersions: Int = 0,
)

/** Respons-body fra `GET /databases/{id}/changes?since=N`. */
@Serializable
data class ChangesResponse(
    @SerialName("current_seq") val currentSeq: Long,
    val entries: List<EntryChange> = emptyList(),
)

/** Body til `PUT /databases/{id}/entries/{uuid}`. */
@Serializable
data class PutEntryRequest(
    val blob: String,
    @SerialName("modified_at") val modifiedAt: String,
)

/** Body til `DELETE /databases/{id}/entries/{uuid}` — blob optional. */
@Serializable
data class DeleteEntryRequest(
    @SerialName("modified_at") val modifiedAt: String,
)

/** Respons fra PUT/DELETE — wrapped i `{"entry": {...}}` på wire-niveau. */
@Serializable
data class EntryPutEnvelope(val entry: EntryPutResponse)

@Serializable
data class EntryPutResponse(
    val uuid: String,
    @SerialName("modified_at") val modifiedAt: String,
    val deleted: Boolean = false,
    val seq: Long,
    @SerialName("created_at") val createdAt: String,
)

/** Database-metadata returneret af `/databases`. */
@Serializable
data class Database(
    val id: String,
    val name: String,
    @SerialName("created_at") val createdAt: String,
    val role: String = "owner",
    @SerialName("wrapped_master_key") val wrappedMasterKey: String? = null,
)

@Serializable
data class DatabaseListResponse(val databases: List<Database> = emptyList())

/** Server-side fejlrepr — wrapped i {"error":{...}}. */
@Serializable
data class ErrorEnvelope(val error: ApiErrorBody)

@Serializable
data class ApiErrorBody(
    val code: String = "",
    val message: String = "",
)
