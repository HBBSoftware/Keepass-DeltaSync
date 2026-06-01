// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

/**
 * Callback som sync-koden kalder undervejs så UI-laget kan vise fremdrift.
 *
 * Den kaldes synkront fra den tråd der kører [Synchronizer.sync]/[SyncEngine.sync]
 * (typisk en IO-tråd) — implementationen skal selv marshalle til UI-tråden.
 * Default-implementationen overalt er en no-op, så ikke-UI-callers (tests,
 * baggrunds-worker) kan ignorere den uden ekstra argumenter.
 */
fun interface SyncProgressListener {
    fun onProgress(event: SyncProgressEvent)
}

/** En enkelt fremdrifts-hændelse i en sync-cyklus. */
sealed interface SyncProgressEvent {
    /** Åbner og dekrypterer den lokale .kdbx — kan tage et øjeblik for store databaser. */
    object Loading : SyncProgressEvent

    /** Pull-fasen: [current] af [total] server-ændringer behandlet (1-baseret). */
    data class Pulling(val current: Int, val total: Int) : SyncProgressEvent

    /** Skriver den opdaterede .kdbx tilbage til disk. */
    object Writing : SyncProgressEvent

    /** Push-fasen: [current] af [total] lokale ændringer sendt (1-baseret). */
    data class Pushing(val current: Int, val total: Int) : SyncProgressEvent

    /** Sync er færdig. */
    object Done : SyncProgressEvent
}
