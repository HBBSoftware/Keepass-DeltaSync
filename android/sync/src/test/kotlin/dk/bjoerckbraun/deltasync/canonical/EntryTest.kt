// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.canonical

import kotlinx.datetime.Instant
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

class EntryTest {

    /**
     * Verificer at vi kan parse en *præcis* JSON-blob produceret af Go-sidens
     * canonical.Entry. Fixturen regenereres ved at køre Go-testen:
     *
     *     go test -tags=emit_fixture -run TestEmitFixture ./internal/kdbx/canonical/
     *
     * Hvis denne test fejler er det stærkt signal om schema-drift mellem Go
     * og Kotlin.
     */
    @Test
    fun `parses Go-emitted fixture`() {
        val raw = loadResource("/canonical-entry-fixture.json")
        val entry = CanonicalJson.decodeFromString(Entry.serializer(), raw)

        assertEquals(SchemaVersion, entry.v)
        assertEquals("00010203-0405-0607-0809-0a0b0c0d0e0f", entry.uuid)

        // Times
        assertEquals(Instant.parse("2026-05-01T10:00:00Z"), entry.times.created)
        assertEquals(Instant.parse("2026-05-29T10:00:00Z"), entry.times.modified)
        assertEquals(5, entry.times.usageCount)
        assertEquals(false, entry.times.expires)

        // Strings: kanoniske + custom side-om-side
        assertEquals("GitLab", entry.strings["Title"]?.v)
        assertEquals("hans", entry.strings["UserName"]?.v)
        assertEquals("s3cr3t", entry.strings["Password"]?.v)
        assertEquals(true, entry.strings["Password"]?.protected)
        assertEquals(false, entry.strings["URL"]?.protected)
        assertEquals("", entry.strings["Notes"]?.v)
        assertEquals("tok-12345", entry.strings["API-Token"]?.v)
        assertEquals(true, entry.strings["API-Token"]?.protected)

        // Binaries: base64 → bytes
        assertEquals(1, entry.binaries.size)
        assertEquals("key.pem", entry.binaries[0].name)
        assertTrue(entry.binaries[0].data.contentEquals(byteArrayOf(0x00, 0xff.toByte(), 0x42)))

        // Tags
        assertEquals(listOf("work", "important"), entry.tags)

        // AutoType
        assertNotNull(entry.autotype)
        assertEquals(true, entry.autotype!!.enabled)
        assertEquals("{USERNAME}{TAB}{PASSWORD}{ENTER}", entry.autotype.defaultSequence)

        // QualityCheck — eksplicit emittered af Go når != null
        assertEquals(true, entry.qualityCheck)

        // CustomData med per-item modified
        assertEquals("browser-state", entry.customData["KPXC_ext"]?.v)
        assertEquals(Instant.parse("2026-05-20T14:00:00Z"), entry.customData["KPXC_ext"]?.modified)
    }

    /**
     * Round-trip i Kotlin: serialize → deserialize → samme objekt.
     * Sikrer at vi ikke har emit/parse-asymmetri i Kotlin selv (uafhængigt
     * af Go-kompatibilitet).
     */
    @Test
    fun `kotlin round-trip is stable`() {
        val original = sampleEntry()
        val json = CanonicalJson.encodeToString(Entry.serializer(), original)
        val parsed = CanonicalJson.decodeFromString(Entry.serializer(), json)
        assertEquals(original, parsed)
    }

    /**
     * Verificer at den Kotlin-emitterede JSON kan parses *tilbage* via
     * Go-stil-decoder. Vi efterligner Go's tolerance her ved bare at
     * sikre at vores egen parser bagefter giver samme entry.
     */
    @Test
    fun `kotlin-emitted JSON round-trips through fixture-parser`() {
        val original = sampleEntry()
        val json = CanonicalJson.encodeToString(Entry.serializer(), original)
        val reparsed = CanonicalJson.decodeFromString(Entry.serializer(), json)
        assertEquals(original.uuid, reparsed.uuid)
        assertEquals(original.times, reparsed.times)
        assertEquals(original.strings, reparsed.strings)
        assertEquals(original.binaries, reparsed.binaries)
    }

    /**
     * Forward-compat: en blob med ekstra ukendte felter (simulerer en
     * fremtidig Go-skemaversion der har tilføjet nye felter) skal stadig
     * parses uden fejl. Kun feltet 'v' bumpes hvis vi tilføjer breaking
     * changes; minor additions bør forblive bagudkompatible.
     */
    @Test
    fun `unknown fields are silently ignored`() {
        val rawWithExtra = """
            {
              "v": 1,
              "uuid": "00010203-0405-0607-0809-0a0b0c0d0e0f",
              "times": {
                "created": "2026-05-01T10:00:00Z",
                "modified": "2026-05-29T10:00:00Z",
                "accessed": "2026-05-29T10:00:00Z",
                "expires": false,
                "usage_count": 0,
                "location_changed": "2026-05-01T10:00:00Z",
                "future_field": "not-yet-modeled"
              },
              "icon_id": 0,
              "strings": {
                "Title": {"v": "Hello", "future_attribute": 42}
              },
              "future_top_level_field": [1, 2, 3]
            }
        """.trimIndent()

        val entry = CanonicalJson.decodeFromString(Entry.serializer(), rawWithExtra)
        assertEquals("Hello", entry.strings["Title"]?.v)
    }

    /**
     * Schema-drift-sikring: hvis Go bumper SchemaVersion uden at Kotlin
     * følger med, fejler denne test loudly i CI på Kotlin-siden. Vi tjekker
     * den faktiske JSON-værdi fra fixturen, ikke vores egen konstant — så
     * begge sider skal opdateres for at testen passerer.
     */
    @Test
    fun `schema version matches Go fixture`() {
        val raw = loadResource("/canonical-entry-fixture.json")
        // Lille manuel parse — vi kigger kun på "v":N på top-niveau, for at
        // undgå at en bredere parse-fejl skjuler en simpel version-mismatch.
        val match = Regex(""""v"\s*:\s*(\d+)""").find(raw)
        assertNotNull(match)
        val fixtureVersion = match!!.groupValues[1].toInt()
        assertEquals(
            SchemaVersion, fixtureVersion,
            "Go fixture's schema version ($fixtureVersion) differs from Kotlin's SchemaVersion ($SchemaVersion). " +
                "Sync the Kotlin side by bumping const SchemaVersion in Entry.kt."
        )
    }

    private fun sampleEntry(): Entry = Entry(
        v = SchemaVersion,
        uuid = "00010203-0405-0607-0809-0a0b0c0d0e0f",
        times = Times(
            created = Instant.parse("2026-05-01T10:00:00Z"),
            modified = Instant.parse("2026-05-29T10:00:00Z"),
            accessed = Instant.parse("2026-05-29T10:00:00Z"),
            locationChanged = Instant.parse("2026-05-01T10:00:00Z"),
        ),
        strings = mapOf(
            "Title" to EntryString(v = "Sample"),
            "Password" to EntryString(v = "hunter2", protected = true),
        ),
        binaries = listOf(
            Binary(name = "doc.txt", data = byteArrayOf(0x41, 0x42, 0x43)),
        ),
        tags = listOf("work"),
    )

    private fun loadResource(path: String): String {
        val stream = javaClass.getResourceAsStream(path)
            ?: error("resource not found: $path")
        return stream.bufferedReader().use { it.readText() }
    }
}
