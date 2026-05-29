// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync

import android.content.Intent
import android.os.Bundle
import android.view.View
import android.widget.TextView
import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts
import androidx.lifecycle.lifecycleScope
import app.keemobile.kotpass.cryptography.EncryptedValue
import app.keemobile.kotpass.database.Credentials
import com.google.android.material.button.MaterialButton
import com.google.android.material.card.MaterialCardView
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import com.google.android.material.textfield.TextInputEditText
import com.google.android.material.textfield.TextInputLayout
import dk.bjoerckbraun.deltasync.api.ApiClient
import dk.bjoerckbraun.deltasync.persistence.DatabaseConfigStore
import dk.bjoerckbraun.deltasync.persistence.DataStoreSyncStatePersistence
import dk.bjoerckbraun.deltasync.persistence.KeystoreTokenStore
import dk.bjoerckbraun.deltasync.persistence.SafKdbxFile
import dk.bjoerckbraun.deltasync.sync.GomobileCryptoSession
import dk.bjoerckbraun.deltasync.sync.Synchronizer
import dk.bjoerckbraun.deltasync.sync.TokenStore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/**
 * Hovedindgangs-Activity. Viser nuværende status (enrolled, konfigureret),
 * tilbyder enrollment + setup-flow ved første kørsel, og når begge dele
 * er på plads: en "Sync now"-knap der prompter for masterpassword.
 */
class MainActivity : ComponentActivity() {

    private lateinit var tokenStore: TokenStore
    private lateinit var configStore: DatabaseConfigStore

    private lateinit var statusText: TextView
    private lateinit var versionText: TextView
    private lateinit var enrollButton: MaterialButton
    private lateinit var setupButton: MaterialButton
    private lateinit var syncNowButton: MaterialButton
    private lateinit var unenrollButton: MaterialButton
    private lateinit var serverRequiredCard: MaterialCardView

    private val enrollLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { result -> if (result.resultCode == RESULT_OK) refreshStatus() }

    private val setupLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { result -> if (result.resultCode == RESULT_OK) refreshStatus() }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        tokenStore = KeystoreTokenStore(applicationContext)
        configStore = DatabaseConfigStore(applicationContext)

        statusText = findViewById(R.id.statusText)
        versionText = findViewById(R.id.versionText)
        enrollButton = findViewById(R.id.enrollButton)
        setupButton = findViewById(R.id.setupButton)
        syncNowButton = findViewById(R.id.syncNowButton)
        unenrollButton = findViewById(R.id.unenrollButton)
        serverRequiredCard = findViewById(R.id.serverRequiredCard)

        versionText.text = buildString {
            append("v0.1.0 · canonical schema v")
            append(runCatching { mobile.Mobile.SchemaVersion }.getOrElse { "?" })
        }

        enrollButton.setOnClickListener {
            enrollLauncher.launch(Intent(this, EnrollActivity::class.java))
        }

        setupButton.setOnClickListener {
            setupLauncher.launch(Intent(this, SetupActivity::class.java))
        }

        syncNowButton.setOnClickListener {
            promptPassphraseAndSync()
        }

        unenrollButton.setOnClickListener {
            tokenStore.clear()
            configStore.clear()
            refreshStatus()
        }
    }

    override fun onResume() {
        super.onResume()
        refreshStatus()
    }

    private fun refreshStatus() {
        val credentials = tokenStore.load()
        val config = configStore.load()

        when {
            credentials == null -> {
                statusText.text = getString(R.string.status_not_enrolled)
                enrollButton.visibility = View.VISIBLE
                setupButton.visibility = View.GONE
                syncNowButton.visibility = View.GONE
                unenrollButton.visibility = View.GONE
                // Vis "server required"-card kun når man ikke er enrolled —
                // det er der brugeren har brug for at vide at app'en ikke
                // virker uden en server.
                serverRequiredCard.visibility = View.VISIBLE
            }

            config == null -> {
                statusText.text = buildString {
                    append("Enrolled.\n\n")
                    append("Server: ").append(credentials.serverUrl).append('\n')
                    append("Device: ").append(credentials.deviceId.take(8)).append("…\n\n")
                    append("Database not configured. Tap below to pick a .kdbx and match it to a server database.")
                }
                enrollButton.visibility = View.GONE
                setupButton.visibility = View.VISIBLE
                syncNowButton.visibility = View.GONE
                unenrollButton.visibility = View.VISIBLE
                serverRequiredCard.visibility = View.GONE
            }

            else -> {
                statusText.text = buildString {
                    append("Ready to sync.\n\n")
                    append("Server: ").append(credentials.serverUrl).append('\n')
                    append("Device: ").append(credentials.deviceId.take(8)).append("…\n")
                    append("Local kdbx: ").append(config.kdbxName).append('\n')
                    append("Server db: ").append(config.databaseId.take(8)).append('…')
                }
                enrollButton.visibility = View.GONE
                setupButton.visibility = View.GONE
                syncNowButton.visibility = View.VISIBLE
                syncNowButton.isEnabled = true
                unenrollButton.visibility = View.VISIBLE
                serverRequiredCard.visibility = View.GONE
            }
        }
    }

    private fun promptPassphraseAndSync() {
        val input = TextInputEditText(this).apply {
            inputType = android.text.InputType.TYPE_CLASS_TEXT or
                android.text.InputType.TYPE_TEXT_VARIATION_PASSWORD
            hint = "kdbx master password"
        }
        val layout = TextInputLayout(this).apply {
            setPadding(48, 24, 48, 0)
            addView(input)
        }

        MaterialAlertDialogBuilder(this)
            .setTitle("Sync now")
            .setMessage("Indtast .kdbx-master-password for at dekryptere lokalt.")
            .setView(layout)
            .setNegativeButton(android.R.string.cancel, null)
            .setPositiveButton("Sync") { _, _ ->
                val passphrase = input.text?.toString().orEmpty()
                if (passphrase.isNotEmpty()) runSync(passphrase)
            }
            .show()
    }

    private fun runSync(passphrase: String) {
        syncNowButton.isEnabled = false
        val credentials = tokenStore.load() ?: return
        val config = configStore.load() ?: return

        lifecycleScope.launch {
            val outcome = withContext(Dispatchers.IO) {
                runCatching {
                    val api = ApiClient(credentials.serverUrl, credentials.deviceToken)
                    val crypto = GomobileCryptoSession.open(passphrase.toByteArray(), config.databaseId)
                    val kdbxFile = SafKdbxFile(applicationContext, config.uri)
                    val kdbxCredentials = Credentials.from(EncryptedValue.fromString(passphrase))
                    val sync = Synchronizer(
                        kdbxFile = kdbxFile,
                        credentials = kdbxCredentials,
                        api = api,
                        crypto = crypto,
                        persistence = DataStoreSyncStatePersistence(applicationContext),
                    )
                    try {
                        sync.sync(config.databaseId)
                    } finally {
                        crypto.close()
                    }
                }
            }

            syncNowButton.isEnabled = true

            outcome.fold(
                onSuccess = { result ->
                    MaterialAlertDialogBuilder(this@MainActivity)
                        .setTitle("Sync OK")
                        .setMessage(buildString {
                            append("Pulled: ").append(result.pulledEntries).append(" entries, ")
                            append(result.pulledDeletions).append(" deletions\n")
                            append("Pushed: ").append(result.pushedEntries).append(" entries, ")
                            append(result.pushedDeletions).append(" deletions\n")
                            append("Server seq: ").append(result.newLastSeq)
                        })
                        .setPositiveButton(android.R.string.ok, null)
                        .show()
                },
                onFailure = { e ->
                    MaterialAlertDialogBuilder(this@MainActivity)
                        .setTitle("Sync failed")
                        .setMessage(e.message ?: e::class.simpleName ?: "unknown error")
                        .setPositiveButton(android.R.string.ok, null)
                        .show()
                },
            )
        }
    }
}
