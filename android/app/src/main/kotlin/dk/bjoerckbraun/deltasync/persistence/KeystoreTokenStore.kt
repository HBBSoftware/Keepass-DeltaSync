// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.persistence

import android.content.Context
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import dk.bjoerckbraun.deltasync.sync.TokenStore
import java.util.Base64

/**
 * [TokenStore]-impl der gemmer device-credentials i Android's
 * [EncryptedSharedPreferences], krypteret med en master-key forankret i
 * Android Keystore (TEE/StrongBox hvor tilgængeligt).
 *
 * Filerne ligger som `<app-data>/shared_prefs/deltasync_credentials.xml`
 * og kan kun læses af denne app's UID. Tabt nøgle ved Keystore-reset
 * betyder genstart fra enroll.
 */
class KeystoreTokenStore(context: Context) : TokenStore {

    private val masterKey: MasterKey = MasterKey.Builder(context)
        .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
        .build()

    private val prefs = EncryptedSharedPreferences.create(
        context,
        FILE_NAME,
        masterKey,
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    )

    override fun save(
        serverUrl: String,
        deviceId: String,
        deviceToken: String,
        devicePrivateKey: ByteArray,
    ) {
        prefs.edit()
            .putString(KEY_SERVER_URL, serverUrl.trimEnd('/'))
            .putString(KEY_DEVICE_ID, deviceId)
            .putString(KEY_DEVICE_TOKEN, deviceToken)
            .putString(KEY_DEVICE_PRIVATE_KEY, Base64.getEncoder().encodeToString(devicePrivateKey))
            .apply()
    }

    override fun load(): TokenStore.Credentials? {
        val serverUrl = prefs.getString(KEY_SERVER_URL, null) ?: return null
        val deviceId = prefs.getString(KEY_DEVICE_ID, null) ?: return null
        val deviceToken = prefs.getString(KEY_DEVICE_TOKEN, null) ?: return null
        val privateKeyB64 = prefs.getString(KEY_DEVICE_PRIVATE_KEY, null) ?: return null
        return TokenStore.Credentials(
            serverUrl = serverUrl,
            deviceId = deviceId,
            deviceToken = deviceToken,
            devicePrivateKey = Base64.getDecoder().decode(privateKeyB64),
        )
    }

    override fun clear() {
        prefs.edit().clear().apply()
    }

    companion object {
        private const val FILE_NAME = "deltasync_credentials"
        private const val KEY_SERVER_URL = "server_url"
        private const val KEY_DEVICE_ID = "device_id"
        private const val KEY_DEVICE_TOKEN = "device_token"
        private const val KEY_DEVICE_PRIVATE_KEY = "device_private_key"
    }
}
