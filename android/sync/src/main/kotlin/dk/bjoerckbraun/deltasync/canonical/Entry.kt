// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.canonical

import kotlinx.datetime.Instant
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Den platform-uafhængige entry-repræsentation. Modellerer KDBX4's entry-skema
 * 1:1 og matcher Go-sidens `canonical.Entry` struct (se
 * `client/internal/kdbx/canonical/entry.go`) byte-for-byte over JSON-wiren —
 * fixture-testen verificerer det.
 *
 * Designgrundlag og migration: docs/v3-canonical-entry-format.md.
 */
@Serializable
data class Entry(
    /** Skema-versionen. Skal være [SchemaVersion] ved emission. */
    val v: Int,

    /** Entry'ens UUID i standard hex-format ("01234567-89ab-..."). */
    val uuid: String,

    val times: Times,

    /**
     * Alle entry-felter — både kanoniske (Title, UserName, Password, URL,
     * Notes) og custom-felter brugeren har tilføjet. KDBX skelner ikke
     * mellem dem, og det gør vi heller ikke.
     */
    val strings: Map<String, EntryString> = emptyMap(),

    /** Inline-attachments. KDBX deler dem via en pool pr. database, men på
     *  wiren er hver entry self-contained. */
    val binaries: List<Binary> = emptyList(),

    val tags: List<String> = emptyList(),

    /** KDBX' built-in ikon (0-68). 0 = nøgle. */
    val iconId: Int = 0,

    /** UUID på en custom-ikon-ressource i kdbx' Meta. Tom hvis ikke sat. */
    val customIconUuid: String = "",

    val foregroundColor: String = "",
    val backgroundColor: String = "",
    val overrideUrl: String = "",

    /** Null hvis entry'en ikke har auto-type-konfiguration. */
    val autotype: AutoType? = null,

    /** KeePassXC-specifikt: skal password-quality-checks køres? Null = brug
     *  KeePassXC's default (typisk true). */
    val qualityCheck: Boolean? = null,

    val customData: Map<String, CustomDataItem> = emptyMap(),

    /** UUID på den gruppe entry'en blev flyttet fra (KeePassXC-specifikt).
     *  Tom hvis entry'en ikke er flyttet. */
    val previousParentGroup: String = "",

    /** UUID på den gruppe entry'en aktuelt ligger i (v4 group-sync). Tom =
     *  Root (sentinel — hver enhed mapper den til sin egen Root). Matcher
     *  Go's `canonical.Entry.ParentGroup` (`parent_group`). */
    val parentGroup: String = "",

    /** Historiske versioner — ældste først. Indlejret history er forbudt
     *  (KDBX' eget format gør det også); validerede entries har altid
     *  tom liste på elementer inde i [history]. */
    val history: List<Entry> = emptyList(),
)

/** KDBX' tidsstempler pr. entry. Alle tider er UTC. */
@Serializable
data class Times(
    val created: Instant,
    val modified: Instant,
    val accessed: Instant,
    /** Null når [expires] er false. */
    val expiresAt: Instant? = null,
    val expires: Boolean = false,
    val usageCount: Int = 0,
    val locationChanged: Instant,
)

/**
 * Et enkelt key-value-felt på en entry.
 *
 * @property protected Indikerer om værdien skal memory-protectes (stream-cipher
 *   krypteres i kdbx-filen) ved re-import. Typisk true for Password og tokens.
 */
@Serializable
data class EntryString(
    val v: String,
    val protected: Boolean = false,
)

/** Inline attachment. */
@Serializable
data class Binary(
    val name: String,
    @Serializable(with = Base64ByteArraySerializer::class)
    val data: ByteArray,
) {
    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is Binary) return false
        return name == other.name && data.contentEquals(other.data)
    }

    override fun hashCode(): Int = 31 * name.hashCode() + data.contentHashCode()
}

/** Auto-type-konfigurationen for en entry. */
@Serializable
data class AutoType(
    val enabled: Boolean = true,
    /** 0 = none, 1 = use-clipboard (KDBX-spec). */
    val obfuscation: Int = 0,
    /** Auto-type-template; tom betyder "brug database-default". */
    val defaultSequence: String = "",
    val associations: List<AutoTypeAssociation> = emptyList(),
)

/** Pr.-vindue auto-type-override. */
@Serializable
data class AutoTypeAssociation(
    val window: String,
    val sequence: String,
)

/** Plugin-værdi i [Entry.customData]. */
@Serializable
data class CustomDataItem(
    val v: String,
    val modified: Instant? = null,
)

/**
 * Den aktuelle canonical-skemaversion. Skal stemme overens med Go's
 * `canonical.SchemaVersion` — schema-drift-testen verificerer det.
 */
const val SchemaVersion: Int = 1
