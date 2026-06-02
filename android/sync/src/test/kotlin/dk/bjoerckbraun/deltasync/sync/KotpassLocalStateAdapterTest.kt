// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import app.keemobile.kotpass.cryptography.EncryptedValue
import app.keemobile.kotpass.database.Credentials
import app.keemobile.kotpass.database.KeePassDatabase
import app.keemobile.kotpass.database.findEntries
import app.keemobile.kotpass.database.modifiers.modifyContent
import app.keemobile.kotpass.database.modifiers.modifyMeta
import app.keemobile.kotpass.database.modifiers.modifyParentGroup
import app.keemobile.kotpass.models.DeletedObject
import app.keemobile.kotpass.models.EntryFields
import app.keemobile.kotpass.models.EntryValue
import app.keemobile.kotpass.models.Group
import app.keemobile.kotpass.models.Meta
import app.keemobile.kotpass.models.TimeData
import kotlinx.datetime.Instant
import kotlinx.datetime.toJavaInstant
import org.junit.jupiter.api.Test
import java.util.UUID
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertTrue
import app.keemobile.kotpass.models.Entry as KotpassEntry

class KotpassLocalStateAdapterTest {

    private val credentials = Credentials.from(EncryptedValue.fromString("test-password"))

    @Test
    fun `read empty database yields empty state`() {
        val db = freshDatabase()
        val state = KotpassLocalStateAdapter.read(db)
        assertTrue(state.entries.isEmpty())
        assertEquals(0L, state.lastSeq)
    }

    @Test
    fun `read database with entries maps to LocalState`() {
        val uuid1 = UUID.fromString("11111111-1111-1111-1111-111111111111")
        val uuid2 = UUID.fromString("22222222-2222-2222-2222-222222222222")

        val db = freshDatabase().modifyParentGroup {
            copy(entries = entries + listOf(
                kotpassEntry(uuid1, "First"),
                kotpassEntry(uuid2, "Second"),
            ))
        }

        val state = KotpassLocalStateAdapter.read(db)

        assertEquals(2, state.entries.size)
        val first = state.entries[uuid1.toString()]
        assertNotNull(first)
        assertEquals("First", first.strings["Title"]?.v)
        val second = state.entries[uuid2.toString()]
        assertNotNull(second)
        assertEquals("Second", second.strings["Title"]?.v)
    }

    @Test
    fun `applyToDatabase adds new entry to root group`() {
        val originalDb = freshDatabase()
        val state = LocalState(
            entries = mutableMapOf(
                "33333333-3333-3333-3333-333333333333" to dk.bjoerckbraun.deltasync.canonical.Entry(
                    v = dk.bjoerckbraun.deltasync.canonical.SchemaVersion,
                    uuid = "33333333-3333-3333-3333-333333333333",
                    times = dk.bjoerckbraun.deltasync.canonical.Times(
                        created = Instant.parse("2026-05-01T10:00:00Z"),
                        modified = Instant.parse("2026-05-29T10:00:00Z"),
                        accessed = Instant.parse("2026-05-29T10:00:00Z"),
                        locationChanged = Instant.parse("2026-05-01T10:00:00Z"),
                    ),
                    strings = mapOf(
                        "Title" to dk.bjoerckbraun.deltasync.canonical.EntryString(v = "Added"),
                    ),
                ),
            ),
        )

        val newDb = KotpassLocalStateAdapter.applyToDatabase(state, originalDb)

        val entries = newDb.findEntries { true }.flatMap { (_, list) -> list }
        assertEquals(1, entries.size)
        assertEquals("Added", entries.first().fields["Title"]?.content)
    }

    @Test
    fun `applyToDatabase updates existing entry`() {
        val uuid = UUID.fromString("44444444-4444-4444-4444-444444444444")
        val originalDb = freshDatabase().modifyParentGroup {
            copy(entries = entries + kotpassEntry(uuid, "Original"))
        }

        val state = KotpassLocalStateAdapter.read(originalDb)
        // Modificer entry'en
        val modified = state.entries[uuid.toString()]!!
            .copy(strings = mapOf(
                "Title" to dk.bjoerckbraun.deltasync.canonical.EntryString(v = "Modified"),
            ))
        state.entries[uuid.toString()] = modified

        val newDb = KotpassLocalStateAdapter.applyToDatabase(state, originalDb)

        val entries = newDb.findEntries { true }.flatMap { (_, list) -> list }
        assertEquals(1, entries.size)
        assertEquals("Modified", entries.first().fields["Title"]?.content)
    }

    @Test
    fun `applyToDatabase removes tombstoned entries`() {
        val uuid = UUID.fromString("55555555-5555-5555-5555-555555555555")
        val originalDb = freshDatabase().modifyParentGroup {
            copy(entries = entries + kotpassEntry(uuid, "ToDelete"))
        }

        val state = LocalState(
            tombstones = mutableMapOf(
                uuid.toString() to Instant.parse("2026-05-29T10:00:00Z"),
            ),
        )

        val newDb = KotpassLocalStateAdapter.applyToDatabase(state, originalDb)

        val entries = newDb.findEntries { true }.flatMap { (_, list) -> list }
        assertEquals(0, entries.size)
    }

    @Test
    fun `read after applyToDatabase round-trips entry data`() {
        val uuid = UUID.fromString("66666666-6666-6666-6666-666666666666")
        val originalDb = freshDatabase().modifyParentGroup {
            copy(entries = entries + kotpassEntry(uuid, "RoundTripTitle"))
        }

        val state1 = KotpassLocalStateAdapter.read(originalDb)
        val sameDb = KotpassLocalStateAdapter.applyToDatabase(state1, originalDb)
        val state2 = KotpassLocalStateAdapter.read(sameDb)

        assertEquals(1, state2.entries.size)
        assertEquals(
            "RoundTripTitle",
            state2.entries[uuid.toString()]?.strings?.get("Title")?.v,
        )
    }

    @Test
    fun `read materializes DeletedObjects as tombstones`() {
        val liveUuid = UUID.fromString("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
        val deletedUuid = UUID.fromString("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
        val deletionTime = Instant.parse("2026-05-30T12:00:00Z")

        val db = freshDatabase()
            .modifyParentGroup { copy(entries = entries + kotpassEntry(liveUuid, "Live")) }
            .modifyContent {
                copy(deletedObjects = deletedObjects + DeletedObject(
                    id = deletedUuid,
                    deletionTime = deletionTime.toJavaInstant(),
                ))
            }

        val state = KotpassLocalStateAdapter.read(db)

        assertEquals(1, state.entries.size)
        assertTrue(state.entries.containsKey(liveUuid.toString()))
        assertEquals(1, state.tombstones.size)
        assertEquals(deletionTime, state.tombstones[deletedUuid.toString()])
    }

    @Test
    fun `read synthesizes recycle-bin entries as tombstones`() {
        val liveUuid = UUID.fromString("cccccccc-cccc-cccc-cccc-cccccccccccc")
        val trashedUuid = UUID.fromString("dddddddd-dddd-dddd-dddd-dddddddddddd")
        val recycleBinId = UUID.fromString("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
        val movedAt = Instant.parse("2026-05-31T08:00:00Z")

        val trashed = kotpassEntry(trashedUuid, "Trashed").let {
            it.copy(times = it.times!!.copy(locationChanged = movedAt.toJavaInstant()))
        }
        val db = freshDatabase()
            .modifyParentGroup {
                copy(
                    entries = entries + kotpassEntry(liveUuid, "Live"),
                    groups = groups + Group(
                        uuid = recycleBinId,
                        name = "Recycle Bin",
                        entries = listOf(trashed),
                    ),
                )
            }
            .modifyMeta { copy(recycleBinEnabled = true, recycleBinUuid = recycleBinId) }

        val state = KotpassLocalStateAdapter.read(db)

        // Den levende entry forbliver aktiv; papirkurv-entry'en bliver et tombstone.
        assertEquals(1, state.entries.size)
        assertTrue(state.entries.containsKey(liveUuid.toString()))
        assertFalse(state.entries.containsKey(trashedUuid.toString()))
        assertEquals(movedAt, state.tombstones[trashedUuid.toString()])
    }

    @Test
    fun `read keeps recycle-bin entries active when recycle bin disabled`() {
        val trashedUuid = UUID.fromString("dddddddd-dddd-dddd-dddd-dddddddddddd")
        val recycleBinId = UUID.fromString("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")

        // Recycle bin udpeget men IKKE aktiveret → entries skal forblive aktive.
        val db = freshDatabase()
            .modifyParentGroup {
                copy(groups = groups + Group(
                    uuid = recycleBinId,
                    name = "Recycle Bin",
                    entries = listOf(kotpassEntry(trashedUuid, "NotReallyTrashed")),
                ))
            }
            .modifyMeta { copy(recycleBinEnabled = false, recycleBinUuid = recycleBinId) }

        val state = KotpassLocalStateAdapter.read(db)

        assertEquals(1, state.entries.size)
        assertTrue(state.entries.containsKey(trashedUuid.toString()))
        assertTrue(state.tombstones.isEmpty())
    }

    @Test
    fun `read lets a live entry win over a stale DeletedObject`() {
        val uuid = UUID.fromString("ffffffff-ffff-ffff-ffff-ffffffffffff")

        val db = freshDatabase()
            .modifyParentGroup { copy(entries = entries + kotpassEntry(uuid, "Resurrected")) }
            .modifyContent {
                copy(deletedObjects = deletedObjects + DeletedObject(
                    id = uuid,
                    deletionTime = Instant.parse("2026-05-01T00:00:00Z").toJavaInstant(),
                ))
            }

        val state = KotpassLocalStateAdapter.read(db)

        assertEquals(1, state.entries.size)
        assertTrue(state.entries.containsKey(uuid.toString()))
        assertTrue(state.tombstones.isEmpty())
    }

    // --- Helpers ---

    private fun freshDatabase(): KeePassDatabase =
        KeePassDatabase.Ver4x.create(
            rootName = "Root",
            meta = Meta(generator = "kotpass-test"),
            credentials = credentials,
        )

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
}
