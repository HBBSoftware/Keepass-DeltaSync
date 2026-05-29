// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync

import android.os.Bundle
import android.view.View
import android.widget.ProgressBar
import android.widget.TextView
import androidx.activity.ComponentActivity
import androidx.lifecycle.lifecycleScope
import com.google.android.material.button.MaterialButton
import com.google.android.material.textfield.TextInputEditText
import dk.bjoerckbraun.deltasync.api.ApiException
import dk.bjoerckbraun.deltasync.api.EnrollmentClient
import dk.bjoerckbraun.deltasync.persistence.KeystoreTokenStore
import dk.bjoerckbraun.deltasync.sync.GomobileCryptoSession
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/**
 * Førstegangs enrollment-screen. Brugeren indtaster server-URL,
 * enrollment-token (éngangs, udleveret af serverens administrator) og
 * et valgfrit device-navn. App'en:
 *
 *   1. Genererer en X25519 device-keypair via gomobile.
 *   2. Kalder [EnrollmentClient.enroll] med pub-key + token.
 *   3. Gemmer (server-url, device-id, device-token, device-private-key)
 *      i [KeystoreTokenStore].
 *   4. Returnerer RESULT_OK til MainActivity der refresher status.
 *
 * Selve enrollment-kaldet sker på [Dispatchers.IO]; UI'en viser en
 * progress-spinner indtil resultatet er kommet.
 */
class EnrollActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_enroll)

        val serverUrlInput = findViewById<TextInputEditText>(R.id.serverUrlInput)
        val tokenInput = findViewById<TextInputEditText>(R.id.tokenInput)
        val deviceNameInput = findViewById<TextInputEditText>(R.id.deviceNameInput)
        val submitButton = findViewById<MaterialButton>(R.id.enrollSubmitButton)
        val progress = findViewById<ProgressBar>(R.id.enrollProgress)
        val errorText = findViewById<TextView>(R.id.enrollError)

        submitButton.setOnClickListener {
            val serverUrl = serverUrlInput.text?.toString()?.trim().orEmpty()
            val token = tokenInput.text?.toString()?.trim().orEmpty()
            val deviceName = deviceNameInput.text?.toString()?.trim().orEmpty()

            if (serverUrl.isEmpty() || token.isEmpty()) {
                errorText.text = "Server URL og enrollment token er påkrævet."
                errorText.visibility = View.VISIBLE
                return@setOnClickListener
            }
            if (!serverUrl.startsWith("http://") && !serverUrl.startsWith("https://")) {
                errorText.text = "Server URL skal starte med https:// eller http://"
                errorText.visibility = View.VISIBLE
                return@setOnClickListener
            }

            submitButton.isEnabled = false
            progress.visibility = View.VISIBLE
            errorText.visibility = View.GONE

            lifecycleScope.launch {
                val outcome = runEnrollment(
                    serverUrl = serverUrl,
                    token = token,
                    deviceName = deviceName,
                )

                progress.visibility = View.GONE
                submitButton.isEnabled = true

                when (outcome) {
                    is EnrollmentOutcome.Success -> {
                        setResult(RESULT_OK)
                        finish()
                    }

                    is EnrollmentOutcome.Failure -> {
                        errorText.text = outcome.message
                        errorText.visibility = View.VISIBLE
                    }
                }
            }
        }
    }

    private suspend fun runEnrollment(
        serverUrl: String,
        token: String,
        deviceName: String,
    ): EnrollmentOutcome = withContext(Dispatchers.IO) {
        try {
            // 1. Generér keypair.
            val keypair = GomobileCryptoSession.generateDeviceKeypair()

            // 2. POST /enroll.
            val client = EnrollmentClient(baseUrl = serverUrl)
            val result = client.enroll(
                enrollmentToken = token,
                deviceName = deviceName.ifBlank { null },
                devicePublicKey = keypair.publicKey,
            )

            // 3. Gem credentials.
            KeystoreTokenStore(applicationContext).save(
                serverUrl = serverUrl,
                deviceId = result.deviceId,
                deviceToken = result.deviceToken,
                devicePrivateKey = keypair.privateKey,
            )

            EnrollmentOutcome.Success
        } catch (e: ApiException) {
            EnrollmentOutcome.Failure("Server afviste: ${e.statusCode} ${e.code} ${e.detail}")
        } catch (e: Exception) {
            EnrollmentOutcome.Failure("Enrollment fejlede: ${e.message ?: e::class.simpleName}")
        }
    }

    private sealed class EnrollmentOutcome {
        object Success : EnrollmentOutcome()
        data class Failure(val message: String) : EnrollmentOutcome()
    }
}
