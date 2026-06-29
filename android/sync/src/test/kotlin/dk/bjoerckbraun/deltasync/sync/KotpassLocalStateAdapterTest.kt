// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import app.keemobile.kotpass.constants.GroupOverride
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

    @Test
    fun `read collects groups and sets entry parentGroup`() {
        val rootEntry = UUID.fromString("10000000-0000-0000-0000-000000000001")
        val workId = UUID.fromString("20000000-0000-0000-0000-000000000002")
        val workEntry = UUID.fromString("21000000-0000-0000-0000-000000000021")
        val subId = UUID.fromString("30000000-0000-0000-0000-000000000003")
        val subEntry = UUID.fromString("31000000-0000-0000-0000-000000000031")

        val db = freshDatabase().modifyParentGroup {
            copy(
                entries = entries + kotpassEntry(rootEntry, "RootE"),
                groups = groups + Group(
                    uuid = workId,
                    name = "Work",
                    entries = listOf(kotpassEntry(workEntry, "WorkE")),
                    groups = listOf(
                        Group(
                            uuid = subId,
                            name = "Sub",
                            entries = listOf(kotpassEntry(subEntry, "SubE")),
                        ),
                    ),
                ),
            )
        }

        val state = KotpassLocalStateAdapter.read(db)

        // 3 entries med korrekt parentGroup (Root = sentinel "").
        assertEquals(3, state.entries.size)
        assertEquals("", state.entries[rootEntry.toString()]?.parentGroup)
        assertEquals(workId.toString(), state.entries[workEntry.toString()]?.parentGroup)
        assertEquals(subId.toString(), state.entries[subEntry.toString()]?.parentGroup)

        // 2 grupper (Root emittes ikke); parent-hierarki bevaret.
        assertEquals(2, state.groups.size)
        assertEquals("Work", state.groups[workId.toString()]?.name)
        assertEquals("", state.groups[workId.toString()]?.parentGroup)
        assertEquals(workId.toString(), state.groups[subId.toString()]?.parentGroup)
    }

    @Test
    fun `applyToDatabase builds group tree and places entries`() {
        val workId = "20000000-0000-0000-0000-000000000002"
        val subId = "30000000-0000-0000-0000-000000000003"
        val rootE = "10000000-0000-0000-0000-000000000001"
        val workE = "21000000-0000-0000-0000-000000000021"
        val subE = "31000000-0000-0000-0000-000000000031"

        val state = LocalState(
            groups = mutableMapOf(
                workId to canonicalGroup(workId, parent = "", name = "Work"),
                subId to canonicalGroup(subId, parent = workId, name = "Sub"),
            ),
            entries = mutableMapOf(
                rootE to canonicalEntryP(rootE, parent = "", title = "RootE"),
                workE to canonicalEntryP(workE, parent = workId, title = "WorkE"),
                subE to canonicalEntryP(subE, parent = subId, title = "SubE"),
            ),
        )

        val db = KotpassLocalStateAdapter.applyToDatabase(state, freshDatabase())

        // Læs tilbage og verificér genopbygget hierarki + placering.
        val s2 = KotpassLocalStateAdapter.read(db)
        assertEquals(3, s2.entries.size)
        assertEquals(2, s2.groups.size)
        assertEquals("", s2.entries[rootE]?.parentGroup)
        assertEquals(workId, s2.entries[workE]?.parentGroup)
        assertEquals(subId, s2.entries[subE]?.parentGroup)
        assertEquals("", s2.groups[workId]?.parentGroup)
        assertEquals(workId, s2.groups[subId]?.parentGroup)
    }

    @Test
    fun `applyToDatabase moves entry to another group`() {
        val workId = UUID.fromString("20000000-0000-0000-0000-000000000002")
        val otherId = UUID.fromString("40000000-0000-0000-0000-000000000004")
        val entryId = UUID.fromString("21000000-0000-0000-0000-000000000021")

        val db0 = freshDatabase().modifyParentGroup {
            copy(groups = groups + listOf(
                Group(uuid = workId, name = "Work", entries = listOf(kotpassEntry(entryId, "E"))),
                Group(uuid = otherId, name = "Other"),
            ))
        }

        val state = KotpassLocalStateAdapter.read(db0)
        // Start: entry i Work.
        assertEquals(workId.toString(), state.entries[entryId.toString()]?.parentGroup)
        // Flyt entry til Other i state (som en pull fra en anden enhed ville gøre).
        state.entries[entryId.toString()] = state.entries[entryId.toString()]!!
            .copy(parentGroup = otherId.toString())

        val db = KotpassLocalStateAdapter.applyToDatabase(state, db0)

        val s2 = KotpassLocalStateAdapter.read(db)
        assertEquals(otherId.toString(), s2.entries[entryId.toString()]?.parentGroup)
        // Og entry'en findes kun ét sted.
        assertEquals(1, s2.entries.size)
    }

    @Test
    fun `applyToDatabase resurrects recycle-bin entry without duplicating UUID`() {
        // Regression: en entry der ligger i den lokale papirkurv men er aktiv på
        // serveren (LWW-resurrection). db.findEntries skjuler papirkurven, så den
        // tidligere "findes allerede?"-check så IKKE entry'en og tilføjede en NY
        // kopi ved siden af den i papirkurven → dublet-UUID (KeePassDX flagger det).
        val trashedUuid = UUID.fromString("dddddddd-dddd-dddd-dddd-dddddddddddd")
        val recycleBinId = UUID.fromString("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")

        val db0 = freshDatabase()
            .modifyParentGroup {
                copy(groups = groups + Group(
                    uuid = recycleBinId,
                    name = "Recycle Bin",
                    entries = listOf(kotpassEntry(trashedUuid, "Trashed")),
                ))
            }
            .modifyMeta { copy(recycleBinEnabled = true, recycleBinUuid = recycleBinId) }

        // read() ser entry'en som et tombstone; serveren genopliver den som aktiv.
        val state = KotpassLocalStateAdapter.read(db0)
        assertTrue(state.tombstones.containsKey(trashedUuid.toString()))
        state.tombstones.remove(trashedUuid.toString())
        state.entries[trashedUuid.toString()] = canonicalEntryP(
            trashedUuid.toString(), parent = "", title = "Resurrected",
        )

        val db = KotpassLocalStateAdapter.applyToDatabase(state, db0)

        // Entry'en må kun findes ÉN gang i HELE træet (inkl. papirkurv) — ikke 2.
        assertEquals(1, countOccurrences(db, trashedUuid))
        // Og den ligger nu i Root, ikke i papirkurven.
        assertEquals(1, db.findEntries { true }.flatMap { (_, l) -> l }
            .count { it.uuid == trashedUuid })
    }

    @Test
    fun `applyToDatabase does not duplicate entry in a search-disabled group`() {
        // Regression for det faktiske felt-fund: KeePassDX lægger sine 7
        // skabelon-entries i en gruppe med EnableSearching=false (Meta
        // EntryTemplatesGroup). db.findEntries respekterer GroupOverride og
        // SPRINGER søgnings-deaktiverede grupper over — så den gamle "findes
        // allerede?"-check så aldrig skabelonerne og tilføjede en ny kopi ved
        // hver sync → dublet-UUID (de blev 3x hos brugeren).
        val tmplGroup = UUID.fromString("f6450db6-4ddd-e535-d1c3-7ed445c35aaa")
        val entryId = UUID.fromString("0c4cdb46-5186-e97c-dfb3-78ca0858ec8c")

        val db0 = freshDatabase().modifyParentGroup {
            copy(groups = groups + Group(
                uuid = tmplGroup,
                name = "Skabeloner",
                enableSearching = GroupOverride.Disabled,
                entries = listOf(kotpassEntry(entryId, "Email")),
            ))
        }

        // read() samler entry'en op som aktiv (collectTree springer ikke
        // søgnings-deaktiverede grupper over). En efterfølgende apply af samme
        // state svarer til en pull der leverer entry'en igen.
        val state = KotpassLocalStateAdapter.read(db0)
        assertTrue(state.entries.containsKey(entryId.toString()))

        val db = KotpassLocalStateAdapter.applyToDatabase(state, db0)

        // Må kun findes ÉN gang i HELE træet (findEntries ville ellers maskere
        // dubletten, da den ikke ser den søgnings-deaktiverede gruppe).
        assertEquals(1, countOccurrences(db, entryId))
    }

    @Test
    fun `applyToDatabase removes tombstoned group`() {
        val workId = UUID.fromString("20000000-0000-0000-0000-000000000002")
        val db0 = freshDatabase().modifyParentGroup {
            copy(groups = groups + Group(uuid = workId, name = "Work"))
        }
        val state = LocalState(
            tombstones = mutableMapOf(workId.toString() to Instant.parse("2026-06-01T00:00:00Z")),
        )
        val db = KotpassLocalStateAdapter.applyToDatabase(state, db0)
        assertFalse(KotpassLocalStateAdapter.read(db).groups.containsKey(workId.toString()))
    }

    // --- Helpers ---

    /** Tæl forekomster af [uuid] i HELE gruppetræet, inkl. papirkurven (modsat findEntries). */
    private fun countOccurrences(db: KeePassDatabase, uuid: UUID): Int {
        fun walk(group: Group): Int =
            group.entries.count { it.uuid == uuid } + group.groups.sumOf { walk(it) }
        return walk(db.content.group)
    }

    private fun canonicalGroup(uuid: String, parent: String, name: String) =
        dk.bjoerckbraun.deltasync.canonical.Group(
            v = dk.bjoerckbraun.deltasync.canonical.GroupSchemaVersion,
            uuid = uuid,
            name = name,
            parentGroup = parent,
            times = sampleTimes(),
        )

    private fun canonicalEntryP(uuid: String, parent: String, title: String) =
        dk.bjoerckbraun.deltasync.canonical.Entry(
            v = dk.bjoerckbraun.deltasync.canonical.SchemaVersion,
            uuid = uuid,
            times = sampleTimes(),
            strings = mapOf("Title" to dk.bjoerckbraun.deltasync.canonical.EntryString(v = title)),
            parentGroup = parent,
        )

    private fun sampleTimes() = dk.bjoerckbraun.deltasync.canonical.Times(
        created = Instant.parse("2026-05-01T10:00:00Z"),
        modified = Instant.parse("2026-05-01T10:00:00Z"),
        accessed = Instant.parse("2026-05-01T10:00:00Z"),
        locationChanged = Instant.parse("2026-05-01T10:00:00Z"),
    )

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
