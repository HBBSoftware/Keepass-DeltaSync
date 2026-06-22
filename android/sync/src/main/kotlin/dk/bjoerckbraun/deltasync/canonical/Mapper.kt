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
import kotlinx.datetime.toKotlinInstant
import java.util.UUID
import app.keemobile.kotpass.models.Entry as KotpassEntry
import app.keemobile.kotpass.models.Group as KotpassGroup

/**
 * Konverterer mellem kotpass' typed [KotpassEntry] og vores
 * platform-uafhængige wire-format [Entry]. Sammen med [CanonicalJson] er
 * dette nok til at desktop og Android producerer byte-identiske krypterede
 * blobs for samme logiske entry.
 *
 * Binaries håndteres via injicerede lambda-parametre fordi kotpass' Entry
 * kun holder [BinaryReference]-pointere (hash + navn) — de faktiske bytes
 * lever i KeePassDatabase's binary-pool. Callers wire op til deres egen pool:
 *
 *   ```kotlin
 *   val canonicalEntry = Mapper.toCanonical(kpEntry) { ref ->
 *       database.content.meta.binaries[ref.hash]
 *   }
 *   ```
 *
 * `kotpass`-feltet `qualityCheck` har default `true`, så vi default'er til
 * `null` ved deserialize hvis det ikke er sat — det matcher Go's pointer-
 * `*bool` med omitempty på samme felt.
 */
object Mapper {

    /**
     * Konverter en kotpass-Entry til canonical-format. [binaryLookup] kaldes
     * for hver [BinaryReference] i entry'en og skal returnere den tilsvarende
     * binary-data fra databasens pool. Returnerer null hvis pool'en ikke
     * indeholder hash'et — vi dropper attachment'en stille i så fald
     * (typisk tegn på inkonsistent database-state).
     */
    fun toCanonical(
        entry: KotpassEntry,
        binaryLookup: (BinaryReference) -> ByteArray? = { null },
    ): Entry {
        val times = entry.times?.let { td ->
            Times(
                created = td.creationTime?.toKotlinInstant() ?: epoch,
                modified = td.lastModificationTime?.toKotlinInstant() ?: epoch,
                accessed = td.lastAccessTime?.toKotlinInstant() ?: epoch,
                expiresAt = if (td.expires) td.expiryTime?.toKotlinInstant() else null,
                expires = td.expires,
                usageCount = td.usageCount,
                locationChanged = td.locationChanged?.toKotlinInstant() ?: epoch,
            )
        } ?: defaultTimes()

        // .entries.associate fremfor .mapValues — kotpass' EntryFields
        // overstyrer mapValues til at returnere en ny EntryFields (med
        // EntryValue), så vi kan ikke bruge den direkte til Map<String,EntryString>.
        val strings: Map<String, EntryString> = entry.fields.entries.associate { (k, value) ->
            k to when (value) {
                is EntryValue.Plain -> EntryString(v = value.content, protected = false)
                is EntryValue.Encrypted -> EntryString(v = value.content, protected = true)
            }
        }

        val binaries = entry.binaries.mapNotNull { ref ->
            val data = binaryLookup(ref) ?: return@mapNotNull null
            Binary(name = ref.name, data = data)
        }

        val autotype = entry.autoType?.let { a ->
            AutoType(
                enabled = a.enabled,
                obfuscation = a.obfuscation.ordinal,
                defaultSequence = a.defaultSequence ?: "",
                associations = a.items.map { AutoTypeAssociation(it.window, it.keystrokeSequence) },
            )
        }

        val customData = entry.customData.mapValues { (_, v) ->
            CustomDataItem(v = v.value, modified = v.lastModified?.toKotlinInstant())
        }

        return Entry(
            v = SchemaVersion,
            uuid = entry.uuid.toString(),
            times = times,
            strings = strings,
            binaries = binaries,
            tags = entry.tags,
            iconId = entry.icon.ordinal,
            customIconUuid = entry.customIconUuid?.toString() ?: "",
            foregroundColor = entry.foregroundColor ?: "",
            backgroundColor = entry.backgroundColor ?: "",
            overrideUrl = entry.overrideUrl,
            autotype = autotype,
            qualityCheck = entry.qualityCheck,
            customData = customData,
            previousParentGroup = entry.previousParentGroup?.toString() ?: "",
            history = entry.history.map { toCanonical(it, binaryLookup) },
        )
    }

    /**
     * Konverter en canonical-Entry til kotpass-format. [binaryStore] kaldes
     * for hver inline [Binary] og skal indsætte data'en i databasens pool
     * (typisk via SHA-256-hash) og returnere referencen kotpass' Entry kan
     * holde fast i.
     */
    fun toKotpass(
        entry: Entry,
        binaryStore: (Binary) -> BinaryReference = { ref ->
            throw IllegalStateException("entry has binary ${ref.name} but no binaryStore was supplied")
        },
    ): KotpassEntry {
        val times = TimeData(
            creationTime = entry.times.created.toJavaInstant(),
            lastModificationTime = entry.times.modified.toJavaInstant(),
            lastAccessTime = entry.times.accessed.toJavaInstant(),
            locationChanged = entry.times.locationChanged.toJavaInstant(),
            expiryTime = entry.times.expiresAt?.toJavaInstant(),
            expires = entry.times.expires,
            usageCount = entry.times.usageCount,
        )

        val fields = EntryFields(
            entry.strings.mapValues { (_, s) ->
                if (s.protected) {
                    EntryValue.Encrypted(EncryptedValue.fromString(s.v))
                } else {
                    EntryValue.Plain(s.v)
                }
            }
        )

        val binaries = entry.binaries.map(binaryStore)

        val autoType = entry.autotype?.let { a ->
            AutoTypeData(
                enabled = a.enabled,
                obfuscation = AutoTypeObfuscation.entries.getOrElse(a.obfuscation) {
                    AutoTypeObfuscation.None
                },
                defaultSequence = a.defaultSequence.takeIf { it.isNotEmpty() },
                items = a.associations.map { AutoTypeItem(it.window, it.sequence) },
            )
        }

        val customData = entry.customData.mapValues { (_, v) ->
            CustomDataValue(value = v.v, lastModified = v.modified?.toJavaInstant())
        }

        val icon = PredefinedIcon.entries.getOrElse(entry.iconId) { PredefinedIcon.Key }

        return KotpassEntry(
            uuid = UUID.fromString(entry.uuid),
            icon = icon,
            customIconUuid = entry.customIconUuid.takeIf { it.isNotEmpty() }?.let(UUID::fromString),
            foregroundColor = entry.foregroundColor.takeIf { it.isNotEmpty() },
            backgroundColor = entry.backgroundColor.takeIf { it.isNotEmpty() },
            overrideUrl = entry.overrideUrl,
            times = times,
            autoType = autoType,
            fields = fields,
            tags = entry.tags,
            binaries = binaries,
            history = entry.history.map { toKotpass(it, binaryStore) },
            customData = customData,
            previousParentGroup = entry.previousParentGroup.takeIf { it.isNotEmpty() }
                ?.let(UUID::fromString),
            qualityCheck = entry.qualityCheck ?: true,
        )
    }

    /**
     * Konverter en kotpass-Group til canonical-format (v4 group-sync).
     * [parentUuid] er forældregruppens UUID, "" hvis gruppen ligger direkte i
     * Root (sentinel) — kalderen kender parent fra træ-traverseringen.
     */
    fun groupToCanonical(group: KotpassGroup, parentUuid: String): Group {
        val times = group.times?.let { td ->
            Times(
                created = td.creationTime?.toKotlinInstant() ?: epoch,
                modified = td.lastModificationTime?.toKotlinInstant() ?: epoch,
                accessed = td.lastAccessTime?.toKotlinInstant() ?: epoch,
                expiresAt = if (td.expires) td.expiryTime?.toKotlinInstant() else null,
                expires = td.expires,
                usageCount = td.usageCount,
                locationChanged = td.locationChanged?.toKotlinInstant() ?: epoch,
            )
        } ?: defaultTimes()

        return Group(
            v = GroupSchemaVersion,
            uuid = group.uuid.toString(),
            name = group.name,
            notes = group.notes,
            parentGroup = parentUuid,
            iconId = group.icon.ordinal,
            customIconUuid = group.customIconUuid?.toString() ?: "",
            times = times,
        )
    }

    /**
     * Konverter en canonical-Group til en kotpass-Group med TOMME børn —
     * entries og undergrupper sættes af træ-genopbygningen i
     * KotpassLocalStateAdapter (v4 phase 5d). Øvrige kotpass-felter defaulter.
     */
    fun groupToKotpass(group: Group): KotpassGroup {
        val times = TimeData(
            creationTime = group.times.created.toJavaInstant(),
            lastModificationTime = group.times.modified.toJavaInstant(),
            lastAccessTime = group.times.accessed.toJavaInstant(),
            locationChanged = group.times.locationChanged.toJavaInstant(),
            expiryTime = group.times.expiresAt?.toJavaInstant(),
            expires = group.times.expires,
            usageCount = group.times.usageCount,
        )
        val icon = PredefinedIcon.entries.getOrElse(group.iconId) { PredefinedIcon.Folder }
        return KotpassGroup(
            uuid = UUID.fromString(group.uuid),
            name = group.name,
            notes = group.notes,
            icon = icon,
            customIconUuid = group.customIconUuid.takeIf { it.isNotEmpty() }?.let(UUID::fromString),
            times = times,
        )
    }

    private val epoch = Instant.fromEpochSeconds(0)

    private fun defaultTimes() = Times(
        created = epoch,
        modified = epoch,
        accessed = epoch,
        locationChanged = epoch,
    )
}
