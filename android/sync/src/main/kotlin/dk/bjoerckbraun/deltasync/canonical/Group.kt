// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.canonical

import kotlinx.serialization.Serializable

/**
 * Den platform-uafhængige gruppe-repræsentation til v4 group-sync. Matcher
 * Go-sidens `canonical.Group` struct (`client/internal/kdbx/canonical/entry.go`)
 * over JSON-wiren via [CanonicalJson]'s SnakeCase-strategi (`parent_group`,
 * `icon_id`, `custom_icon_uuid`).
 *
 * Grupper synkroniseres som selvstændige UUID-nøglede objekter (object_kind=2
 * på serveren, envelope-byte 0x02), parallelt med entries. Sletning håndteres
 * via server's tombstone, ikke et felt her. Se docs/v4-group-sync.md.
 */
@Serializable
data class Group(
    /** Skema-versionen. Skal være [GroupSchemaVersion] ved emission. */
    val v: Int,

    /** Gruppens UUID i standard hex-format. */
    val uuid: String,

    val name: String,
    val notes: String = "",

    /** UUID på forældregruppen. Tom = Root (sentinel), samme konvention som
     *  [Entry.parentGroup]. Root-gruppen synkroniseres aldrig som et objekt. */
    val parentGroup: String = "",

    /** KDBX' built-in ikon. */
    val iconId: Int = 0,

    /** UUID på en custom-ikon-ressource. Tom hvis ikke sat. */
    val customIconUuid: String = "",

    val times: Times,
)

/**
 * Canonical-skemaversionen for [Group]-objekter. Bumpes uafhængigt af
 * entry-skemaet ([SchemaVersion]). Skal stemme med Go's `GroupSchemaVersion`.
 */
const val GroupSchemaVersion: Int = 1
