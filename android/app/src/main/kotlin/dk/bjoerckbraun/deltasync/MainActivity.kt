// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync

import android.content.Intent
import android.os.Bundle
import android.widget.TextView
import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts
import com.google.android.material.button.MaterialButton
import dk.bjoerckbraun.deltasync.persistence.KeystoreTokenStore
import dk.bjoerckbraun.deltasync.sync.TokenStore

/**
 * Hovedindgangs-Activity. Viser nuværende status (enrolled / ikke enrolled),
 * tilbyder enrollment-flow ved første kørsel, og når brugeren er enrolled:
 * en "sync now"-knap.
 *
 * v0.1 mangler stadig fil-picker for kdbx + masterpassword-prompt — sync-
 * knappen er disabled indtil disse er på plads.
 */
class MainActivity : ComponentActivity() {

    private lateinit var tokenStore: TokenStore
    private lateinit var statusText: TextView
    private lateinit var versionText: TextView
    private lateinit var enrollButton: MaterialButton
    private lateinit var syncNowButton: MaterialButton
    private lateinit var unenrollButton: MaterialButton

    private val enrollLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { result ->
        if (result.resultCode == RESULT_OK) {
            // Genindlæs status — bruger er nu enrolled.
            refreshStatus()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        tokenStore = KeystoreTokenStore(applicationContext)

        statusText = findViewById(R.id.statusText)
        versionText = findViewById(R.id.versionText)
        enrollButton = findViewById(R.id.enrollButton)
        syncNowButton = findViewById(R.id.syncNowButton)
        unenrollButton = findViewById(R.id.unenrollButton)

        versionText.text = buildString {
            append("v0.1.0 · canonical schema v")
            append(runCatching { mobile.Mobile.SchemaVersion }.getOrElse { "?" })
            append(" · JNI ")
            append(runCatching { mobile.Mobile::class.java.simpleName }.fold(
                onSuccess = { "loaded" },
                onFailure = { "FAILED: ${it.message}" },
            ))
        }

        enrollButton.setOnClickListener {
            enrollLauncher.launch(Intent(this, EnrollActivity::class.java))
        }

        unenrollButton.setOnClickListener {
            tokenStore.clear()
            refreshStatus()
        }

        syncNowButton.setOnClickListener {
            // TODO: kræver kdbx-picker + masterpassword-prompt før der kan
            // syncs. For nu er knappen disabled; vi viser bare en placeholder.
        }
    }

    override fun onResume() {
        super.onResume()
        refreshStatus()
    }

    private fun refreshStatus() {
        val credentials = tokenStore.load()
        if (credentials == null) {
            statusText.text = getString(R.string.status_not_enrolled)
            enrollButton.visibility = android.view.View.VISIBLE
            syncNowButton.visibility = android.view.View.GONE
            unenrollButton.visibility = android.view.View.GONE
        } else {
            statusText.text = buildString {
                append("Enrolled.\n\n")
                append("Server: ")
                append(credentials.serverUrl)
                append("\nDevice ID: ")
                append(credentials.deviceId.take(8))
                append("…\n\n")
                append("Sync setup not yet implemented — pick a .kdbx file in a later release.")
            }
            enrollButton.visibility = android.view.View.GONE
            syncNowButton.visibility = android.view.View.VISIBLE
            syncNowButton.isEnabled = false
            unenrollButton.visibility = android.view.View.VISIBLE
        }
    }
}
