// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

import mobile.DeviceKeypair
import mobile.Mobile
import mobile.Session as GoSession

/**
 * Production [CryptoSession]-implementation der wrapper den gomobile-bound
 * `mobile.Session` fra `../libs/deltasync.aar`. JNI-laget er kun aktivt
 * når koden kører på Android (de prebuilt'e `.so`-filer er for
 * armeabi-v7a / arm64-v8a / x86 / x86_64); på pure JVM vil klassebindingen
 * kompilere men `Mobile.newSession` vil fejle på `_init()` JNI-load.
 *
 * Brug factory-metoderne [open] (owner) eller [openWithMasterKey] (member
 * efter unwrap af `wrapped_master_key`) — direkte konstruktion er muligt
 * men obskure.
 */
class GomobileCryptoSession internal constructor(
    private val session: GoSession,
) : CryptoSession {

    @Volatile
    private var closed = false

    override fun encryptEntry(entryJson: ByteArray): ByteArray {
        check(!closed) { "session closed" }
        return session.encryptEntry(entryJson)
    }

    override fun decryptEntry(blob: ByteArray): ByteArray {
        check(!closed) { "session closed" }
        return session.decryptEntry(blob)
    }

    override fun encryptGroup(groupJson: ByteArray): ByteArray {
        check(!closed) { "session closed" }
        // INTEGRATION-GAP (v4 group-sync): den prebuilt .aar har endnu ikke
        // Session.EncryptGroup (envelope-byte 0x02). Når mobile.go er udvidet
        // og .aar regenereret, erstat denne throw med `session.encryptGroup(...)`.
        // Indtil da er gruppe-sync kun aktiv i :sync-unit-tests (fake CryptoSession).
        throw NotImplementedError("group encryption requires a rebuilt gomobile .aar (mobile.Session.EncryptGroup)")
    }

    override fun decryptGroup(blob: ByteArray): ByteArray {
        check(!closed) { "session closed" }
        // INTEGRATION-GAP: se encryptGroup — kræver .aar-regen med Session.DecryptGroup.
        throw NotImplementedError("group decryption requires a rebuilt gomobile .aar (mobile.Session.DecryptGroup)")
    }

    override fun close() {
        if (closed) return
        closed = true
        session.close()
    }

    companion object {
        /**
         * Owner-side bootstrap: derive entry-encryption-nøglen via Argon2id
         * over masterpassword + database-ID. Argon2id koster ~200ms, så
         * caller'en bør ikke wrappe dette i en suspend-funktion uden
         * baggrunds-dispatcher.
         *
         * Password-bytes ejes af caller'en og skal zero'es efter kaldet.
         */
        fun open(password: ByteArray, databaseId: String): GomobileCryptoSession =
            GomobileCryptoSession(Mobile.newSession(password, databaseId))

        /**
         * Member-side bootstrap: konstruer en session direkte fra et
         * allerede-unwrapped `master_key` (se [unwrapSharedMasterKey]).
         *
         * Parametre-orden afspejler `client/mobile/mobile.go`'s
         * `NewSessionFromMasterKey(databaseID string, masterKey []byte)` —
         * byttet for at undgå Java-signature-kollision med [open].
         */
        fun openWithMasterKey(databaseId: String, masterKey: ByteArray): GomobileCryptoSession =
            GomobileCryptoSession(Mobile.newSessionFromMasterKey(databaseId, masterKey))

        /**
         * X25519-keypair til v2 sharing. Public-delen postes ved enrollment;
         * private-delen gemmes lokalt i Android Keystore /
         * EncryptedSharedPreferences. Begge byte-arrays er 32 bytes.
         */
        fun generateDeviceKeypair(): GoKeypair {
            val kp = Mobile.generateDeviceKeypair()
            return GoKeypair(kp.publicKey, kp.privateKey)
        }

        /**
         * Bob's side af sealed-box: unwrap `wrapped_master_key` (downloadet
         * fra serveren) med Bob's device-keypair. Resultatet er den
         * faktiske `master_key` der kan fodres til [openWithMasterKey].
         */
        fun unwrapSharedMasterKey(
            wrappedBlob: ByteArray,
            devicePublicKey: ByteArray,
            devicePrivateKey: ByteArray,
        ): ByteArray = Mobile.unwrapSharedMasterKey(wrappedBlob, devicePublicKey, devicePrivateKey)

        /**
         * Genskab device public-key fra dens private-key — bruges når
         * Android-laget kun har gemt private-delen (sparer plads og er
         * det rene minimum for at unwrap-stien virker).
         */
        fun publicKeyFromPrivate(privateKey: ByteArray): ByteArray =
            Mobile.publicKeyFromPrivate(privateKey)

        /**
         * Owner-side af sealed-box (modstykket til [unwrapSharedMasterKey]):
         * derive database master_key fra masterpassword (Argon2id ~200ms) og
         * wrap det til [targetPublicKey] (modtagerens device public-key). Det
         * resulterende opaque blob uploades som `wrapped_master_key`. Kald fra
         * en baggrunds-dispatcher pga. Argon2id-omkostningen. Password-bytes
         * ejes af caller'en.
         */
        fun wrapMasterKeyForShare(
            password: ByteArray,
            databaseId: String,
            targetPublicKey: ByteArray,
        ): ByteArray = Mobile.wrapMasterKeyForShare(password, databaseId, targetPublicKey)

        /**
         * Det canonical-skema-version Go-siden emitterer i nye blobs.
         * Bør matche `dk.bjoerckbraun.deltasync.canonical.SchemaVersion`;
         * en mismatch indikerer at .aar'en og :sync-modulet er ude af sync.
         */
        val schemaVersion: Long get() = Mobile.SchemaVersion
    }
}

/** Pakket X25519-keypair returneret af [GomobileCryptoSession.generateDeviceKeypair]. */
data class GoKeypair(
    val publicKey: ByteArray,
    val privateKey: ByteArray,
) {
    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is GoKeypair) return false
        return publicKey.contentEquals(other.publicKey) && privateKey.contentEquals(other.privateKey)
    }

    override fun hashCode(): Int =
        31 * publicKey.contentHashCode() + privateKey.contentHashCode()
}

/**
 * Tjek at canonical-skemaversion-konstanten i :sync's Kotlin-side matcher
 * det den gomobile-bound .aar emitterer. Kaldes typisk én gang ved app-
 * start eller fra en CI-test.
 */
fun assertSchemaVersionsMatch() {
    val kotlin = dk.bjoerckbraun.deltasync.canonical.SchemaVersion.toLong()
    val go = GomobileCryptoSession.schemaVersion
    check(kotlin == go) {
        "schema version drift: Kotlin canonical=$kotlin, Go gomobile=$go — re-build the .aar or bump Kotlin's SchemaVersion"
    }
}
