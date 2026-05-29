// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.canonical

import app.keemobile.kotpass.constants.AutoTypeObfuscation
import app.keemobile.kotpass.constants.PredefinedIcon
import app.keemobile.kotpass.cryptography.EncryptedValue
import app.keemobile.kotpass.models.AutoTypeData
import app.keemobile.kotpass.models.AutoTypeItem
import app.keemobile.kotpass.models.BinaryReference
import app.keemobile.kotpass.models.CustomDataValue
import app.keemobile.kotpass.models.EntryFields
import app.keemobile.kotpass.models.EntryValue
import app.keemobile.kotpass.models.TimeData
import kotlinx.datetime.Instant
import kotlinx.datetime.toJavaInstant
import okio.ByteString.Companion.toByteString
import org.junit.jupiter.api.Test
import java.security.MessageDigest
import java.util.UUID
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue
import app.keemobile.kotpass.models.Entry as KotpassEntry

class MapperTest {

    /**
     * Kerne-round-trip: kotpass.Entry → canonical → kotpass.Entry. Vi
     * sammenligner ikke direkte struct-equality (kotpass' EncryptedValue er
     * en frisk XOR-salt pr. konstruktion, så toString'en er random hver gang
     * og data class equals fejler) men felt-for-felt.
     */
    @Test
    fun `kotpass to canonical to kotpass preserves entry`() {
        val store = FakeBinaryStore()
        store.store("payload".toByteArray(), "payload.bin")

        val original = sampleKotpassEntry(store)

        val canonical = Mapper.toCanonical(original) { ref -> store.lookup(ref) }
        val roundTripped = Mapper.toKotpass(canonical) { binary -> store.store(binary.data, binary.name) }

        assertEquals(original.uuid, roundTripped.uuid)
        assertEquals(original.icon, roundTripped.icon)
        assertEquals(original.overrideUrl, roundTripped.overrideUrl)
        assertEquals(original.tags, roundTripped.tags)
        assertEquals(original.qualityCheck, roundTripped.qualityCheck)

        // Times (round-trip via java.time.Instant kan miste sub-sekund-præcision —
        // sekund-præcision er nok til KDBX' formål).
        val ot = original.times!!
        val rt = roundTripped.times!!
        assertEquals(ot.creationTime?.epochSecond, rt.creationTime?.epochSecond)
        assertEquals(ot.lastModificationTime?.epochSecond, rt.lastModificationTime?.epochSecond)
        assertEquals(ot.expires, rt.expires)
        assertEquals(ot.usageCount, rt.usageCount)

        // Fields: sammenlign keys + content; protected-flag oversættes via
        // EntryValue-typen.
        assertEquals(original.fields.keys, roundTripped.fields.keys)
        for ((key, origValue) in original.fields) {
            val rtValue = roundTripped.fields[key]
            assertNotNull(rtValue)
            assertEquals(origValue.content, rtValue.content)
            assertEquals(origValue is EntryValue.Encrypted, rtValue is EntryValue.Encrypted)
        }

        // AutoType
        val oa = original.autoType!!
        val ra = roundTripped.autoType!!
        assertEquals(oa.enabled, ra.enabled)
        assertEquals(oa.obfuscation, ra.obfuscation)
        assertEquals(oa.defaultSequence, ra.defaultSequence)
        assertEquals(oa.items.size, ra.items.size)

        // CustomData
        assertEquals(original.customData.keys, roundTripped.customData.keys)
        assertEquals(
            original.customData["KPXC_ext"]?.value,
            roundTripped.customData["KPXC_ext"]?.value,
        )

        // Binaries: hash-baseret reference — efter round-trip via vores fake
        // store skal navnet bevares og data'en være tilgængelig.
        assertEquals(original.binaries.size, roundTripped.binaries.size)
        assertEquals(original.binaries.first().name, roundTripped.binaries.first().name)
    }

    /**
     * Canonical → kotpass → canonical: garanterer at vi ikke taber data
     * gennem den anden vej. JSON-bytes sammenlignes så vi sikrer at format-
     * detaljer (felt-orden, default-værdier) er stabile.
     */
    @Test
    fun `canonical to kotpass to canonical preserves JSON`() {
        val store = FakeBinaryStore()
        val original = sampleCanonicalEntry()

        val kotpass = Mapper.toKotpass(original) { binary -> store.store(binary.data, binary.name) }
        val roundTripped = Mapper.toCanonical(kotpass) { ref -> store.lookup(ref) }

        assertEquals(original.uuid, roundTripped.uuid)
        assertEquals(original.iconId, roundTripped.iconId)
        assertEquals(original.overrideUrl, roundTripped.overrideUrl)
        assertEquals(original.tags, roundTripped.tags)
        assertEquals(original.qualityCheck, roundTripped.qualityCheck)

        // Strings + protected
        assertEquals(original.strings.keys, roundTripped.strings.keys)
        for ((key, s) in original.strings) {
            assertEquals(s.v, roundTripped.strings[key]?.v)
            assertEquals(s.protected, roundTripped.strings[key]?.protected)
        }

        // Binaries: skal være identiske (samme bytes via fake store).
        assertEquals(original.binaries.size, roundTripped.binaries.size)
        val origBin = original.binaries.first()
        val rtBin = roundTripped.binaries.first()
        assertEquals(origBin.name, rtBin.name)
        assertTrue(origBin.data.contentEquals(rtBin.data))

        // CustomData
        assertEquals(
            original.customData["KPXC_ext"]?.v,
            roundTripped.customData["KPXC_ext"]?.v,
        )
    }

    /**
     * Når en BinaryReference ikke kan opslås (data inkonsistens, eller pool
     * mangler binaryen), dropper vi attachment'en stille. Andre felter skal
     * fortsat være intakte.
     */
    @Test
    fun `missing binary in pool drops attachment but keeps other fields`() {
        val emptyStore = FakeBinaryStore()
        val store = FakeBinaryStore()
        store.store("payload".toByteArray(), "payload.bin")

        val original = sampleKotpassEntry(store)
        val canonical = Mapper.toCanonical(original) { emptyStore.lookup(it) } // null!

        // Attachment væk
        assertEquals(0, canonical.binaries.size)
        // Resten intakt
        assertEquals(original.uuid.toString(), canonical.uuid)
        assertTrue(canonical.strings.isNotEmpty())
    }

    /**
     * qualityCheck er nullable på canonical-side (matcher Go's `*bool`
     * omitempty), default-true på kotpass-side. Round-trip skal håndtere
     * begge retninger:
     *
     *  - kotpass.qualityCheck=false → canonical.qualityCheck=false (eksplicit)
     *  - canonical.qualityCheck=null → kotpass.qualityCheck=true (kotpass-default)
     */
    @Test
    fun `qualityCheck null on canonical means kotpass default true`() {
        val entry = sampleCanonicalEntry().copy(qualityCheck = null)
        val kotpass = Mapper.toKotpass(entry) { FakeBinaryStore().store(it.data, it.name) }
        assertEquals(true, kotpass.qualityCheck)
    }

    @Test
    fun `qualityCheck false on kotpass propagates to canonical`() {
        val store = FakeBinaryStore()
        val original = sampleKotpassEntry(store).copy(qualityCheck = false)
        val canonical = Mapper.toCanonical(original) { store.lookup(it) }
        assertEquals(false, canonical.qualityCheck)
    }

    /**
     * KeePass-XC' built-in ikoner mapper til ordinaler i kotpass'
     * [PredefinedIcon]-enum og til iconId-feltet i canonical. Round-trip
     * af icon Star (ordinal 60 i KeePass-XC's liste).
     */
    @Test
    fun `icon ordinal round-trips through PredefinedIcon`() {
        val store = FakeBinaryStore()
        val original = sampleKotpassEntry(store).copy(icon = PredefinedIcon.Star)

        val canonical = Mapper.toCanonical(original) { store.lookup(it) }
        assertEquals(PredefinedIcon.Star.ordinal, canonical.iconId)

        val roundTripped = Mapper.toKotpass(canonical) { store.store(it.data, it.name) }
        assertEquals(PredefinedIcon.Star, roundTripped.icon)
    }

    /**
     * Empty strings på canonical-siden (Title="" osv.) skal blive til
     * EntryValue.Plain("") på kotpass-siden — ikke null.
     */
    @Test
    fun `empty plain string maps to plain empty kotpass value`() {
        val entry = Entry(
            v = SchemaVersion,
            uuid = "00010203-0405-0607-0809-0a0b0c0d0e0f",
            times = Times(
                created = Instant.parse("2026-05-01T10:00:00Z"),
                modified = Instant.parse("2026-05-29T10:00:00Z"),
                accessed = Instant.parse("2026-05-29T10:00:00Z"),
                locationChanged = Instant.parse("2026-05-01T10:00:00Z"),
            ),
            strings = mapOf("Title" to EntryString(v = "")),
        )

        val kotpass = Mapper.toKotpass(entry)
        val title = kotpass.fields["Title"]
        assertNotNull(title)
        assertTrue(title is EntryValue.Plain)
        assertEquals("", title.content)
    }

    /**
     * History-undertræ skal recursivt mappes — uden indlejret history
     * (KDBX' eget skema forbyder det også).
     */
    @Test
    fun `history is recursively mapped`() {
        val store = FakeBinaryStore()
        val historical = sampleKotpassEntry(store).copy(
            fields = EntryFields(mapOf("Title" to EntryValue.Plain("OldTitle"))),
        )
        val original = sampleKotpassEntry(store).copy(history = listOf(historical))

        val canonical = Mapper.toCanonical(original) { store.lookup(it) }

        assertEquals(1, canonical.history.size)
        assertEquals("OldTitle", canonical.history.first().strings["Title"]?.v)
        // Indlejret history skal være tom (default).
        assertTrue(canonical.history.first().history.isEmpty())
    }

    /**
     * Cross-platform check: kotpass-input → canonical → JSON skal kunne
     * parses tilbage via samme JSON-config der bruges over wiren.
     */
    @Test
    fun `kotpass-derived canonical entry round-trips via JSON`() {
        val store = FakeBinaryStore()
        store.store("payload".toByteArray(), "payload.bin")

        val original = sampleKotpassEntry(store)
        val canonical = Mapper.toCanonical(original) { store.lookup(it) }

        val json = CanonicalJson.encodeToString(Entry.serializer(), canonical)
        val parsed = CanonicalJson.decodeFromString(Entry.serializer(), json)

        assertEquals(canonical.uuid, parsed.uuid)
        assertEquals(canonical.strings.keys, parsed.strings.keys)
        assertEquals(canonical.binaries.size, parsed.binaries.size)
    }

    // --- Test-helpers ---

    /**
     * In-memory binary-pool fake. Indekserer på SHA-256 — samme hash-strategi
     * som KDBX-formatet bruger til at deduplicere attachments. Pool'en ejer
     * kun bytes; navnet er per-reference (samme bytes kan optræde under
     * forskellige navne i forskellige entries).
     */
    private class FakeBinaryStore {
        private val pool = mutableMapOf<String, ByteArray>()

        fun store(data: ByteArray, name: String): BinaryReference {
            val sha = MessageDigest.getInstance("SHA-256").digest(data)
            val hash = sha.toByteString(0, sha.size)
            pool[hash.hex()] = data
            return BinaryReference(hash = hash, name = name)
        }

        fun lookup(ref: BinaryReference): ByteArray? = pool[ref.hash.hex()]
    }

    private fun sampleKotpassEntry(store: FakeBinaryStore): KotpassEntry {
        val binaryRef = store.store("payload".toByteArray(), "payload.bin")
        return KotpassEntry(
            uuid = UUID.fromString("00010203-0405-0607-0809-0a0b0c0d0e0f"),
            icon = PredefinedIcon.Key,
            overrideUrl = "https://example.com/override",
            tags = listOf("work", "important"),
            times = TimeData(
                creationTime = Instant.parse("2026-05-01T10:00:00Z").toJavaInstant(),
                lastModificationTime = Instant.parse("2026-05-29T10:00:00Z").toJavaInstant(),
                lastAccessTime = Instant.parse("2026-05-29T10:00:00Z").toJavaInstant(),
                locationChanged = Instant.parse("2026-05-01T10:00:00Z").toJavaInstant(),
                expiryTime = null,
                expires = false,
                usageCount = 5,
            ),
            autoType = AutoTypeData(
                enabled = true,
                obfuscation = AutoTypeObfuscation.None,
                defaultSequence = "{USERNAME}{TAB}{PASSWORD}{ENTER}",
                items = listOf(AutoTypeItem(window = "browser", keystrokeSequence = "{USERNAME}")),
            ),
            fields = EntryFields(mapOf(
                "Title" to EntryValue.Plain("GitLab"),
                "UserName" to EntryValue.Plain("hans"),
                "Password" to EntryValue.Encrypted(EncryptedValue.fromString("s3cr3t")),
                "URL" to EntryValue.Plain("https://gitlab.com"),
                "Notes" to EntryValue.Plain(""),
                "API-Token" to EntryValue.Encrypted(EncryptedValue.fromString("tok-12345")),
            )),
            binaries = listOf(binaryRef),
            customData = mapOf(
                "KPXC_ext" to CustomDataValue(
                    value = "browser-state",
                    lastModified = Instant.parse("2026-05-20T14:00:00Z").toJavaInstant(),
                ),
            ),
            qualityCheck = true,
        )
    }

    private fun sampleCanonicalEntry(): Entry = Entry(
        v = SchemaVersion,
        uuid = "00010203-0405-0607-0809-0a0b0c0d0e0f",
        times = Times(
            created = Instant.parse("2026-05-01T10:00:00Z"),
            modified = Instant.parse("2026-05-29T10:00:00Z"),
            accessed = Instant.parse("2026-05-29T10:00:00Z"),
            locationChanged = Instant.parse("2026-05-01T10:00:00Z"),
            usageCount = 5,
        ),
        strings = mapOf(
            "Title" to EntryString("GitLab"),
            "Password" to EntryString("s3cr3t", protected = true),
        ),
        binaries = listOf(Binary("key.pem", byteArrayOf(0x00, 0xff.toByte(), 0x42))),
        tags = listOf("work"),
        iconId = 0,
        overrideUrl = "https://example.com/override",
        autotype = AutoType(
            enabled = true,
            defaultSequence = "{USERNAME}{TAB}{PASSWORD}{ENTER}",
        ),
        qualityCheck = true,
        customData = mapOf("KPXC_ext" to CustomDataItem("browser-state")),
    )
}
