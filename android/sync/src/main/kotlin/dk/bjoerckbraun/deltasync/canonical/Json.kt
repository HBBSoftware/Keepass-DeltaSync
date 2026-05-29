// SPDX-License-Identifier: GPL-3.0-or-later
@file:OptIn(kotlinx.serialization.ExperimentalSerializationApi::class)

package dk.bjoerckbraun.deltasync.canonical

import kotlinx.serialization.KSerializer
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.descriptors.SerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonNamingStrategy
import java.util.Base64

/**
 * Den canonical Json-konfiguration. Brug denne overalt der serialiseres mod
 * wire-formatet — ikke en default Json() — fordi felt-navne, defaults og
 * unknown-key-behandling er essentielle for cross-platform-kompatibilitet
 * med Go-siden.
 *
 * - **SnakeCase**: matcher Go's struct-tags (`created_at`, `usage_count`, ...)
 *   uden at vi skal annotere hvert eneste felt med @SerialName.
 * - **encodeDefaults**: vi emitterer alle felter, inklusive default-værdier
 *   (matcher Go's "always-emit hvis ikke omitempty"). Output kan være lidt
 *   mere verbose end Go's, men Go's decoder er tolerant.
 * - **explicitNulls=false**: nullable felter med null-værdi (Instant? = null,
 *   Boolean? = null) udelades fra output — matcher Go's `omitempty` på
 *   pointer-typer.
 * - **ignoreUnknownKeys**: forward-compat med fremtidige Go-skemaversioner
 *   der tilføjer nye felter. Vi læser hvad vi forstår og dropper resten.
 */
val CanonicalJson: Json = Json {
    namingStrategy = JsonNamingStrategy.SnakeCase
    encodeDefaults = true
    explicitNulls = false
    ignoreUnknownKeys = true
}

/**
 * Custom serializer der koder [ByteArray] som base64-string. Match Go's
 * `encoding/json` behaviour for `[]byte`, hvor Kotlin's default
 * ellers ville emittere en JSON-array af integers.
 *
 * Bruges kun via `@Serializable(with = Base64ByteArraySerializer::class)`
 * på de specifikke felter der har brug for det (fx [Binary.data]).
 */
object Base64ByteArraySerializer : KSerializer<ByteArray> {
    override val descriptor: SerialDescriptor =
        PrimitiveSerialDescriptor("Base64ByteArray", PrimitiveKind.STRING)

    override fun serialize(encoder: Encoder, value: ByteArray) {
        encoder.encodeString(Base64.getEncoder().encodeToString(value))
    }

    override fun deserialize(decoder: Decoder): ByteArray {
        return Base64.getDecoder().decode(decoder.decodeString())
    }
}
