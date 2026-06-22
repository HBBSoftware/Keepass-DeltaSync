// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.canonical

import kotlinx.datetime.Instant
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class GroupTest {

    private fun sampleGroup(): Group = Group(
        v = GroupSchemaVersion,
        uuid = "11112222-3333-4444-5555-666677778888",
        name = "Work",
        notes = "Arbejdsrelaterede logins",
        parentGroup = "00010203-0405-0607-0809-0a0b0c0d0e0f",
        iconId = 48,
        times = Times(
            created = Instant.parse("2026-05-01T10:00:00Z"),
            modified = Instant.parse("2026-05-29T10:00:00Z"),
            accessed = Instant.parse("2026-05-29T10:00:00Z"),
            locationChanged = Instant.parse("2026-05-01T10:00:00Z"),
        ),
    )

    @Test
    fun `kotlin round-trip is stable`() {
        val original = sampleGroup()
        val json = CanonicalJson.encodeToString(Group.serializer(), original)
        val parsed = CanonicalJson.decodeFromString(Group.serializer(), json)
        assertEquals(original, parsed)
    }

    @Test
    fun `emits snake_case keys matching Go`() {
        val json = CanonicalJson.encodeToString(Group.serializer(), sampleGroup())
        assertTrue(json.contains("\"parent_group\""), "expected parent_group key, got: $json")
        assertTrue(json.contains("\"icon_id\""), "expected icon_id key")
        assertTrue(json.contains("\"custom_icon_uuid\""), "expected custom_icon_uuid key")
    }

    @Test
    fun `parses Go-style group JSON`() {
        val raw = """
            {
              "v": 1,
              "uuid": "11112222-3333-4444-5555-666677778888",
              "name": "Work",
              "parent_group": "00010203-0405-0607-0809-0a0b0c0d0e0f",
              "icon_id": 48,
              "times": {
                "created": "2026-05-01T10:00:00Z",
                "modified": "2026-05-29T10:00:00Z",
                "accessed": "2026-05-29T10:00:00Z",
                "expires": false,
                "usage_count": 0,
                "location_changed": "2026-05-01T10:00:00Z"
              }
            }
        """.trimIndent()
        val g = CanonicalJson.decodeFromString(Group.serializer(), raw)
        assertEquals("Work", g.name)
        assertEquals("00010203-0405-0607-0809-0a0b0c0d0e0f", g.parentGroup)
        assertEquals(48, g.iconId)
        assertEquals(GroupSchemaVersion, g.v)
    }

    @Test
    fun `entry parent_group round-trips`() {
        val entry = Entry(
            v = SchemaVersion,
            uuid = "00010203-0405-0607-0809-0a0b0c0d0e0f",
            times = Times(
                created = Instant.parse("2026-05-01T10:00:00Z"),
                modified = Instant.parse("2026-05-01T10:00:00Z"),
                accessed = Instant.parse("2026-05-01T10:00:00Z"),
                locationChanged = Instant.parse("2026-05-01T10:00:00Z"),
            ),
            parentGroup = "11112222-3333-4444-5555-666677778888",
        )
        val json = CanonicalJson.encodeToString(Entry.serializer(), entry)
        assertTrue(json.contains("\"parent_group\""), "entry should emit parent_group: $json")
        val parsed = CanonicalJson.decodeFromString(Entry.serializer(), json)
        assertEquals(entry.parentGroup, parsed.parentGroup)
    }
}
