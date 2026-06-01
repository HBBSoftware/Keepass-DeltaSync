// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync

import android.content.Intent
import android.os.Bundle
import android.view.View
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.TextView
import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts
import androidx.lifecycle.lifecycleScope
import app.keemobile.kotpass.cryptography.EncryptedValue
import app.keemobile.kotpass.database.Credentials
import com.google.android.material.button.MaterialButton
import com.google.android.material.card.MaterialCardView
import com.google.android.material.checkbox.MaterialCheckBox
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import com.google.android.material.textfield.TextInputEditText
import com.google.android.material.textfield.TextInputLayout
import dk.bjoerckbraun.deltasync.api.ApiClient
import dk.bjoerckbraun.deltasync.persistence.DatabaseConfigStore
import dk.bjoerckbraun.deltasync.persistence.DataStoreSyncStatePersistence
import dk.bjoerckbraun.deltasync.persistence.KeystoreTokenStore
import dk.bjoerckbraun.deltasync.persistence.SafKdbxFile
import dk.bjoerckbraun.deltasync.sync.GomobileCryptoSession
import dk.bjoerckbraun.deltasync.sync.SyncProgressEvent
import dk.bjoerckbraun.deltasync.sync.SyncProgressListener
import dk.bjoerckbraun.deltasync.sync.Synchronizer
import dk.bjoerckbraun.deltasync.sync.TokenStore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/**
 * Hovedindgangs-Activity. Viser nuværende status (enrolled, konfigureret),
 * tilbyder enrollment + setup-flow ved første kørsel, og når begge dele
 * er på plads: en "Sync now"-knap der prompter for masterpassword.
 *
 * Alle bruger-synlige strenge ligger i strings.xml så UI'en kan vises
 * på engelsk (default) eller dansk (values-da/) afhængig af enheds-sproget.
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
    private lateinit var syncProgressBar: ProgressBar
    private lateinit var syncProgressText: TextView

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
        syncProgressBar = findViewById(R.id.syncProgressBar)
        syncProgressText = findViewById(R.id.syncProgressText)

        val schemaVersion = runCatching { mobile.Mobile.SchemaVersion.toInt() }.getOrElse { 0 }
        versionText.text = getString(R.string.version_footer, BuildConfig.VERSION_NAME, schemaVersion)

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
            SessionPassphrase.clear()
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
                serverRequiredCard.visibility = View.VISIBLE
            }

            config == null -> {
                statusText.text = getString(
                    R.string.status_enrolled_no_db_format,
                    credentials.serverUrl,
                    credentials.deviceId.take(8),
                )
                enrollButton.visibility = View.GONE
                setupButton.visibility = View.VISIBLE
                syncNowButton.visibility = View.GONE
                unenrollButton.visibility = View.VISIBLE
                serverRequiredCard.visibility = View.GONE
            }

            else -> {
                statusText.text = getString(
                    R.string.status_ready_format,
                    credentials.serverUrl,
                    credentials.deviceId.take(8),
                    config.kdbxName,
                    config.databaseId.take(8),
                )
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
        // Husket fra en tidligere sync i samme session → spring dialogen over.
        SessionPassphrase.get()?.let { runSync(it); return }

        val input = TextInputEditText(this).apply {
            inputType = android.text.InputType.TYPE_CLASS_TEXT or
                android.text.InputType.TYPE_TEXT_VARIATION_PASSWORD
            hint = getString(R.string.sync_dialog_hint)
        }
        val inputLayout = TextInputLayout(this).apply { addView(input) }
        val rememberCheckbox = MaterialCheckBox(this).apply {
            text = getString(R.string.sync_remember_password)
            setPadding(0, 24, 0, 0)
        }
        val container = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(48, 24, 48, 0)
            addView(inputLayout)
            addView(rememberCheckbox)
        }

        MaterialAlertDialogBuilder(this)
            .setTitle(R.string.sync_dialog_title)
            .setMessage(R.string.sync_dialog_message)
            .setView(container)
            .setNegativeButton(android.R.string.cancel, null)
            .setPositiveButton(R.string.sync_dialog_action) { _, _ ->
                val passphrase = input.text?.toString().orEmpty()
                if (passphrase.isNotEmpty()) {
                    if (rememberCheckbox.isChecked) SessionPassphrase.remember(passphrase)
                    runSync(passphrase)
                }
            }
            .show()
    }

    private fun runSync(passphrase: String) {
        val credentials = tokenStore.load() ?: return
        val config = configStore.load() ?: return
        syncNowButton.isEnabled = false
        showProgress()

        // Listener kaldes fra IO-tråden — marshallér til UI-tråden.
        val progress = SyncProgressListener { event ->
            runOnUiThread { renderProgress(event) }
        }

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
                        progress = progress,
                    )
                    try {
                        sync.sync(config.databaseId)
                    } finally {
                        crypto.close()
                    }
                }
            }

            syncNowButton.isEnabled = true
            hideProgress()

            outcome.fold(
                onSuccess = { result ->
                    MaterialAlertDialogBuilder(this@MainActivity)
                        .setTitle(R.string.sync_result_ok_title)
                        .setMessage(getString(
                            R.string.sync_result_format,
                            result.pulledEntries,
                            result.pulledDeletions,
                            result.pushedEntries,
                            result.pushedDeletions,
                            result.newLastSeq,
                        ))
                        .setPositiveButton(android.R.string.ok, null)
                        .show()
                },
                onFailure = { e ->
                    // Et husket password må ikke blive hængende efter en fejl —
                    // var det forkert (eller filen ændret), skal næste forsøg
                    // prompte igen i stedet for at fejle i en løkke.
                    SessionPassphrase.clear()
                    MaterialAlertDialogBuilder(this@MainActivity)
                        .setTitle(R.string.sync_result_failed_title)
                        .setMessage(e.message ?: e::class.simpleName ?: "unknown error")
                        .setPositiveButton(android.R.string.ok, null)
                        .show()
                },
            )
        }
    }

    private fun showProgress() {
        syncProgressBar.visibility = View.VISIBLE
        syncProgressText.visibility = View.VISIBLE
        syncProgressBar.isIndeterminate = true
        syncProgressText.text = getString(R.string.sync_progress_starting)
    }

    private fun hideProgress() {
        syncProgressBar.visibility = View.GONE
        syncProgressText.visibility = View.GONE
    }

    private fun renderProgress(event: SyncProgressEvent) {
        when (event) {
            is SyncProgressEvent.Loading -> {
                syncProgressBar.isIndeterminate = true
                syncProgressText.text = getString(R.string.sync_progress_opening)
            }
            is SyncProgressEvent.Pulling -> {
                syncProgressBar.isIndeterminate = false
                syncProgressBar.max = event.total
                syncProgressBar.progress = event.current
                syncProgressText.text =
                    getString(R.string.sync_progress_pulling, event.current, event.total)
            }
            is SyncProgressEvent.Writing -> {
                syncProgressBar.isIndeterminate = true
                syncProgressText.text = getString(R.string.sync_progress_saving)
            }
            is SyncProgressEvent.Pushing -> {
                syncProgressBar.isIndeterminate = false
                syncProgressBar.max = event.total
                syncProgressBar.progress = event.current
                syncProgressText.text =
                    getString(R.string.sync_progress_pushing, event.current, event.total)
            }
            is SyncProgressEvent.Done -> {
                syncProgressBar.isIndeterminate = true
                syncProgressText.text = getString(R.string.sync_progress_finishing)
            }
        }
    }
}
