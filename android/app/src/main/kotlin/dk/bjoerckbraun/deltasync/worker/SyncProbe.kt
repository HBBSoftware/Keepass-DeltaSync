// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.worker

import dk.bjoerckbraun.deltasync.api.ApiClient
import dk.bjoerckbraun.deltasync.persistence.FileFingerprint
import java.io.IOException

/**
 * Den billige "er der overhovedet noget at gøre?"-genvej der lader både
 * baggrunds-workeren og foreground-sync springe den dyre kdbx-dekodning
 * (to Argon2-operationer) over når intet er ændret.
 */
object SyncProbe {

    /**
     * Sandt kun hvis der beviseligt er INTET at synke:
     *   (a) den lokale fil er uændret siden sidste sync (samme [current] som
     *       det gemte [saved] fingeraftryk), OG
     *   (b) serveren har ingen seq højere end vores [lastSeq].
     *
     * Tjekker filen FØRST (intet netværk); rammer kun serveren hvis filen er
     * uændret. Et manglende fingeraftryk (`null`, fx fra en provider uden
     * metadata, eller før første sync) tæller som "ændret" → ingen skip, så
     * vi aldrig springer en reel ændring over.
     *
     * Kaster [IOException] hvis server-proben ikke kunne nås — caller'en
     * bestemmer selv: baggrund retrier, foreground er stille.
     */
    @Throws(IOException::class)
    fun nothingToSync(
        api: ApiClient,
        databaseId: String,
        lastSeq: Long,
        current: FileFingerprint?,
        saved: FileFingerprint?,
    ): Boolean {
        if (current == null || saved == null || current != saved) return false
        val head = api.getChanges(databaseId, lastSeq)
        return head.currentSeq == lastSeq && head.entries.isEmpty()
    }
}
