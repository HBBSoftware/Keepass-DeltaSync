// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync

import android.os.Bundle
import android.view.View
import android.widget.ProgressBar
import android.widget.TextView
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.lifecycle.lifecycleScope
import com.google.android.material.button.MaterialButton
import com.google.android.material.textfield.TextInputEditText
import com.journeyapps.barcodescanner.ScanContract
import com.journeyapps.barcodescanner.ScanOptions
import dk.bjoerckbraun.deltasync.api.ApiException
import dk.bjoerckbraun.deltasync.api.EnrollmentClient
import dk.bjoerckbraun.deltasync.enroll.EnrollUriParser
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

    private lateinit var serverUrlInput: TextInputEditText
    private lateinit var tokenInput: TextInputEditText
    private lateinit var deviceNameInput: TextInputEditText
    private lateinit var errorText: TextView

    // ZXing scan-launcher: åbner kamera-scanneren og leverer den rå QR-tekst
    // (eller null hvis brugeren annullerer). Registreres i onCreate.
    private val scanLauncher = registerForActivityResult(ScanContract()) { result ->
        val contents = result.contents ?: return@registerForActivityResult // annulleret
        onScanned(contents)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_enroll)

        serverUrlInput = findViewById(R.id.serverUrlInput)
        tokenInput = findViewById(R.id.tokenInput)
        deviceNameInput = findViewById(R.id.deviceNameInput)
        errorText = findViewById(R.id.enrollError)
        val submitButton = findViewById<MaterialButton>(R.id.enrollSubmitButton)
        val scanButton = findViewById<MaterialButton>(R.id.enrollScanButton)
        val progress = findViewById<ProgressBar>(R.id.enrollProgress)

        scanButton.setOnClickListener {
            errorText.visibility = View.GONE
            scanLauncher.launch(ScanOptions().apply {
                setDesiredBarcodeFormats(ScanOptions.QR_CODE)
                setPrompt(getString(R.string.enroll_scan_prompt))
                setBeepEnabled(false)
                setOrientationLocked(false)
            })
        }

        submitButton.setOnClickListener {
            val serverUrl = serverUrlInput.text?.toString()?.trim().orEmpty()
            val token = tokenInput.text?.toString()?.trim().orEmpty()
            val deviceName = deviceNameInput.text?.toString()?.trim().orEmpty()

            if (serverUrl.isEmpty() || token.isEmpty()) {
                errorText.setText(R.string.enroll_error_required_fields)
                errorText.visibility = View.VISIBLE
                return@setOnClickListener
            }
            if (!serverUrl.startsWith("http://") && !serverUrl.startsWith("https://")) {
                errorText.setText(R.string.enroll_error_url_scheme)
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

    /**
     * Håndterer en scannet QR: parser `deltasync://enroll`-URI'en og udfylder
     * felterne. Vi auto-submitter bevidst IKKE — brugeren får lov at se
     * server-URL'en (bekræfte at det er den rigtige server) og evt. tilføje et
     * enheds-navn før der trykkes Enroll. Ugyldige koder afvises pænt.
     */
    private fun onScanned(contents: String) {
        val parsed = EnrollUriParser.parse(contents)
        if (parsed == null) {
            errorText.setText(R.string.enroll_scan_invalid)
            errorText.visibility = View.VISIBLE
            return
        }
        serverUrlInput.setText(parsed.server)
        tokenInput.setText(parsed.token)
        parsed.deviceName?.let { deviceNameInput.setText(it) }
        errorText.visibility = View.GONE
        Toast.makeText(this, R.string.enroll_scan_filled, Toast.LENGTH_SHORT).show()
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
            EnrollmentOutcome.Failure(getString(R.string.enroll_error_server_format,
                e.statusCode, e.code, e.detail))
        } catch (e: Exception) {
            EnrollmentOutcome.Failure(getString(R.string.enroll_error_generic_format,
                e.message ?: e::class.simpleName ?: ""))
        }
    }

    private sealed class EnrollmentOutcome {
        object Success : EnrollmentOutcome()
        data class Failure(val message: String) : EnrollmentOutcome()
    }
}
