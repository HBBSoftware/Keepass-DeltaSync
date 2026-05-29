// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.sync

/**
 * Sikkert vedvarende lager for device-credentials: device-token (HTTP
 * auth) og device-private-key (X25519, til sealed-box unwrap af shared
 * databaser).
 *
 * Android-impl bruger Android Keystore + EncryptedSharedPreferences;
 * tests bruger en in-memory impl. Tab af denne data betyder at brugeren
 * må re-enrolle.
 */
interface TokenStore {

    /**
     * Gem device-credentials efter succesfuld enrollment.
     *
     * @param serverUrl Server-rod-URL (uden trailing slash).
     * @param deviceId Server-side UUID for denne enhed.
     * @param deviceToken Permanent HTTP-token.
     * @param devicePrivateKey 32-byte X25519 private-key.
     */
    fun save(
        serverUrl: String,
        deviceId: String,
        deviceToken: String,
        devicePrivateKey: ByteArray,
    )

    /** Returnerer credentials hvis enrolled, ellers null. */
    fun load(): Credentials?

    /**
     * Slet alt — bruges ved logout eller hvis serveren returnerer 401
     * (token revoked).
     */
    fun clear()

    /** Pakket device-credentials returneret af [load]. */
    data class Credentials(
        val serverUrl: String,
        val deviceId: String,
        val deviceToken: String,
        val devicePrivateKey: ByteArray,
    ) {
        override fun equals(other: Any?): Boolean {
            if (this === other) return true
            if (other !is Credentials) return false
            return serverUrl == other.serverUrl
                && deviceId == other.deviceId
                && deviceToken == other.deviceToken
                && devicePrivateKey.contentEquals(other.devicePrivateKey)
        }

        override fun hashCode(): Int {
            var result = serverUrl.hashCode()
            result = 31 * result + deviceId.hashCode()
            result = 31 * result + deviceToken.hashCode()
            result = 31 * result + devicePrivateKey.contentHashCode()
            return result
        }
    }
}

/**
 * Thread-safe in-memory impl af [TokenStore] til tests. Værdier
 * persisterer ikke over JVM-genstarter; produktion bruger
 * `AndroidKeystoreTokenStore` (i `:app`-modulet, ikke skrevet endnu).
 */
class InMemoryTokenStore : TokenStore {
    private var current: TokenStore.Credentials? = null

    @Synchronized
    override fun save(
        serverUrl: String,
        deviceId: String,
        deviceToken: String,
        devicePrivateKey: ByteArray,
    ) {
        current = TokenStore.Credentials(
            serverUrl = serverUrl.trimEnd('/'),
            deviceId = deviceId,
            deviceToken = deviceToken,
            devicePrivateKey = devicePrivateKey.copyOf(),
        )
    }

    @Synchronized
    override fun load(): TokenStore.Credentials? = current

    @Synchronized
    override fun clear() {
        // Best-effort nuller key-materiale før release. Ægte zeroing kræver
        // sub.tle.zeroize-tricks; for in-memory test-impl er dette OK.
        current?.devicePrivateKey?.fill(0)
        current = null
    }
}
