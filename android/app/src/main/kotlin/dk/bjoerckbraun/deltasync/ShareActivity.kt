// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync

import dk.bjoerckbraun.deltasync.ui.applySystemAndImeInsets
import android.os.Bundle
import android.util.Base64
import android.view.View
import android.content.DialogInterface
import android.graphics.Typeface
import android.widget.CheckBox
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.TextView
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.lifecycle.lifecycleScope
import com.google.android.material.button.MaterialButton
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import com.google.android.material.textfield.TextInputEditText
import com.google.android.material.textfield.TextInputLayout
import dk.bjoerckbraun.deltasync.api.ApiClient
import dk.bjoerckbraun.deltasync.api.ApiException
import dk.bjoerckbraun.deltasync.api.ShareMember
import dk.bjoerckbraun.deltasync.persistence.DatabaseConfigStore
import dk.bjoerckbraun.deltasync.persistence.EncryptedPassphraseStore
import dk.bjoerckbraun.deltasync.persistence.KeystoreTokenStore
import dk.bjoerckbraun.deltasync.persistence.KnownKeyStore
import dk.bjoerckbraun.deltasync.sync.GomobileCryptoSession
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/**
 * Ejer-side af v2 sharing: del den konfigurerede database med andre brugere,
 * se nuværende medlemmer, og fjern adgang igen. Tilgås fra MainActivity når
 * enheden er enrolled + konfigureret.
 *
 * Flow ved deling (spejler desktop'ens `share`-kommando):
 *   1. Slå brugeren op (`/users/lookup`) → få target-enhedens public-key.
 *   2. Skaf masterpasswordet (fra [EncryptedPassphraseStore] hvis husket,
 *      ellers prompt).
 *   3. `wrapMasterKeyForShare` deriverer master_key (Argon2id) og wrap'er
 *      det som sealed-box til target-pubkey'en.
 *   4. POST `/databases/{id}/shares` med det opaque wrapped blob.
 *
 * Server håndhæver owner-only; er vi ikke owner svarer den 403 og vi viser
 * en pæn besked + deaktiverer del-formularen.
 */
class ShareActivity : ComponentActivity() {

    private lateinit var configStore: DatabaseConfigStore
    private lateinit var passphraseStore: EncryptedPassphraseStore
    private lateinit var knownKeys: KnownKeyStore
    private lateinit var api: ApiClient
    private lateinit var databaseId: String

    private lateinit var usernameInput: TextInputEditText
    private lateinit var shareActionButton: MaterialButton
    private lateinit var memberList: LinearLayout
    private lateinit var progress: ProgressBar
    private lateinit var errorText: TextView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_share)
        applySystemAndImeInsets()

        val credentials = KeystoreTokenStore(applicationContext).load()
        configStore = DatabaseConfigStore(applicationContext)
        passphraseStore = EncryptedPassphraseStore(applicationContext)
        knownKeys = KnownKeyStore(applicationContext)
        val config = configStore.load()
        if (credentials == null || config == null) {
            // Burde ikke ske — knappen vises kun når begge er på plads.
            finish()
            return
        }
        databaseId = config.databaseId
        api = ApiClient(credentials.serverUrl, credentials.deviceToken)

        usernameInput = findViewById(R.id.usernameInput)
        shareActionButton = findViewById(R.id.shareActionButton)
        memberList = findViewById(R.id.memberList)
        progress = findViewById(R.id.shareProgress)
        errorText = findViewById(R.id.shareError)

        shareActionButton.setOnClickListener { onShareClicked() }

        loadMembers()
    }

    private fun loadMembers() {
        progress.visibility = View.VISIBLE
        errorText.visibility = View.GONE
        memberList.removeAllViews()

        lifecycleScope.launch {
            val outcome = withContext(Dispatchers.IO) {
                try {
                    Outcome.Members(api.listShares(databaseId))
                } catch (e: ApiException) {
                    Outcome.Error(e.statusCode, e.detail)
                } catch (e: Exception) {
                    Outcome.Error(0, e.message ?: e::class.simpleName ?: "unknown error")
                }
            }

            progress.visibility = View.GONE
            when (outcome) {
                is Outcome.Members -> renderMembers(outcome.members)
                is Outcome.Error -> {
                    // 403 = ikke owner → del-formularen giver ingen mening.
                    if (outcome.statusCode == 403) {
                        setShareFormEnabled(false)
                        showError(getString(R.string.share_not_owner))
                    } else {
                        showError(outcome.message)
                    }
                }
            }
        }
    }

    private fun renderMembers(members: List<ShareMember>) {
        memberList.removeAllViews()
        if (members.isEmpty()) {
            showError(getString(R.string.share_no_members))
            return
        }
        for (member in members) {
            memberList.addView(memberRow(member))
        }
    }

    private fun memberRow(member: ShareMember): View {
        val row = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = android.view.Gravity.CENTER_VERTICAL
            setPadding(0, 12, 0, 12)
        }
        val label = TextView(this).apply {
            val display = member.displayName?.takeIf { it.isNotBlank() }
            text = if (display != null) {
                getString(R.string.share_member_line_named, display, member.username, member.role)
            } else {
                getString(R.string.share_member_line, member.username, member.role)
            }
            textSize = 16f
            layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
        }
        row.addView(label)

        // Owners kan ikke fjernes (databasen skal have en ejer); kun members.
        if (member.role != "owner") {
            val remove = MaterialButton(
                this, null,
                com.google.android.material.R.attr.materialButtonOutlinedStyle,
            ).apply {
                text = getString(R.string.share_remove)
                setOnClickListener { confirmRemove(member) }
            }
            row.addView(remove)
        }
        return row
    }

    private fun confirmRemove(member: ShareMember) {
        MaterialAlertDialogBuilder(this)
            .setTitle(R.string.share_remove_title)
            .setMessage(getString(R.string.share_remove_confirm, member.username))
            .setNegativeButton(android.R.string.cancel, null)
            .setPositiveButton(R.string.share_remove) { _, _ -> removeMember(member) }
            .show()
    }

    private fun removeMember(member: ShareMember) {
        progress.visibility = View.VISIBLE
        lifecycleScope.launch {
            val error = withContext(Dispatchers.IO) {
                try {
                    api.unshareDatabase(databaseId, member.userId)
                    null
                } catch (e: ApiException) {
                    getString(R.string.share_error_format, e.statusCode, e.detail)
                } catch (e: Exception) {
                    e.message ?: e::class.simpleName ?: "unknown error"
                }
            }
            progress.visibility = View.GONE
            if (error != null) {
                toast(error)
            } else {
                toast(getString(R.string.share_removed, member.username))
            }
            loadMembers()
        }
    }

    private fun onShareClicked() {
        val username = usernameInput.text?.toString()?.trim().orEmpty()
        if (username.isEmpty()) {
            usernameInput.error = getString(R.string.share_username_required)
            return
        }
        withMasterPassword { password -> doShare(username, password) }
    }

    /**
     * Leverer masterpasswordet til [onPassword]: brug det huskede (Keystore-
     * krypterede) hvis det findes, ellers prompt. Vi gemmer IKKE et indtastet
     * password her — det er kun til den enkelte share-handling.
     */
    private fun withMasterPassword(onPassword: (String) -> Unit) {
        passphraseStore.load(databaseId)?.let { onPassword(it); return }

        val input = TextInputEditText(this).apply {
            inputType = android.text.InputType.TYPE_CLASS_TEXT or
                android.text.InputType.TYPE_TEXT_VARIATION_PASSWORD
            hint = getString(R.string.sync_dialog_hint)
        }
        val layout = TextInputLayout(this).apply {
            setPadding(48, 24, 48, 0)
            addView(input)
        }
        MaterialAlertDialogBuilder(this)
            .setTitle(R.string.share_password_title)
            .setMessage(R.string.share_password_message)
            .setView(layout)
            .setNegativeButton(android.R.string.cancel, null)
            .setPositiveButton(android.R.string.ok) { _, _ ->
                val pw = input.text?.toString().orEmpty()
                if (pw.isNotEmpty()) onPassword(pw)
            }
            .show()
    }

    /**
     * Del i tre trin. Modtagerens public-key kommer fra serveren, og en
     * kompromitteret server kan levere sin egen — så vi wrapper kun til en
     * nøgle brugeren har godkendt:
     *
     *  1. Slå modtageren op og regn nøglens aftryk.
     *  2. Kendt og uændret nøgle ([KnownKeyStore]) → videre. Ny eller ændret
     *     → dialog med aftrykket; Del-knappen er slået fra indtil brugeren har
     *     krydset af at aftrykket er sammenlignet med modtageren.
     *  3. Wrap og del — til præcis den nøgle der blev vist.
     */
    private fun doShare(username: String, password: String) {
        setBusy(true)

        lifecycleScope.launch {
            val looked = withContext(Dispatchers.IO) {
                try {
                    val lookup = api.lookupUser(username)
                    val targetPub = Base64.decode(lookup.targetDevice.publicKey, Base64.DEFAULT)
                    Recipient(
                        userId = lookup.user.id,
                        username = lookup.user.username,
                        device = lookup.targetDevice.name,
                        publicKey = targetPub,
                        fingerprint = GomobileCryptoSession.keyFingerprint(targetPub),
                        keyId = GomobileCryptoSession.keyId(targetPub),
                    )
                } catch (e: ApiException) {
                    ShareOutcome.Failure(getString(R.string.share_error_format, e.statusCode, e.detail))
                } catch (e: Exception) {
                    ShareOutcome.Failure(e.message ?: e::class.simpleName ?: "unknown error")
                }
            }
            when (looked) {
                is ShareOutcome.Failure -> {
                    setBusy(false)
                    showError(looked.message)
                }
                is Recipient -> {
                    val known = knownKeys.load(looked.username)
                    if (known == looked.keyId) {
                        completeShare(looked, password)
                    } else {
                        setBusy(false)
                        confirmKey(looked, changed = known != null) { completeShare(looked, password) }
                    }
                }
                else -> setBusy(false)
            }
        }
    }

    private fun completeShare(recipient: Recipient, password: String) {
        setBusy(true)
        lifecycleScope.launch {
            val outcome = withContext(Dispatchers.IO) {
                try {
                    val wrapped = GomobileCryptoSession.wrapMasterKeyForShare(
                        password.toByteArray(),
                        databaseId,
                        recipient.publicKey,
                    )
                    api.shareDatabase(databaseId, recipient.userId, wrapped)
                    knownKeys.save(recipient.username, recipient.keyId)
                    ShareOutcome.Success(recipient.username)
                } catch (e: ApiException) {
                    ShareOutcome.Failure(getString(R.string.share_error_format, e.statusCode, e.detail))
                } catch (e: Exception) {
                    ShareOutcome.Failure(e.message ?: e::class.simpleName ?: "unknown error")
                }
            }

            setBusy(false)
            when (outcome) {
                is ShareOutcome.Success -> {
                    usernameInput.text?.clear()
                    toast(getString(R.string.share_shared_with, outcome.username))
                    loadMembers()
                }
                is ShareOutcome.Failure -> showError(outcome.message)
            }
        }
    }

    /**
     * Viser modtagerens aftryk og kræver et flueben før der kan deles. Et
     * aftryk man kan trykke sig forbi uden at have kigget på, beskytter ikke
     * mod noget — derfor er knappen slået fra indtil da.
     */
    private fun confirmKey(recipient: Recipient, changed: Boolean, onConfirm: () -> Unit) {
        val pad = (20 * resources.displayMetrics.density).toInt()
        val message = TextView(this).apply {
            text = getString(
                if (changed) R.string.share_key_changed else R.string.share_key_new,
                recipient.username,
            )
            if (changed) setTextColor(getColor(android.R.color.holo_red_dark))
        }
        val device = TextView(this).apply {
            text = getString(R.string.share_key_device, recipient.device)
            setPadding(0, pad, 0, 0)
        }
        val fingerprint = TextView(this).apply {
            text = recipient.fingerprint
            typeface = Typeface.MONOSPACE
            textSize = 20f
            setTextIsSelectable(true)
            setPadding(0, pad / 2, 0, pad / 2)
        }
        val compared = CheckBox(this).apply {
            text = getString(R.string.share_key_compared, recipient.username)
        }
        val layout = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(pad + pad / 5, pad / 2, pad + pad / 5, 0)
            addView(message)
            addView(device)
            addView(fingerprint)
            addView(compared)
        }
        val dialog = MaterialAlertDialogBuilder(this)
            .setTitle(getString(R.string.share_key_title, recipient.username))
            .setView(android.widget.ScrollView(this).apply { addView(layout) })
            .setNegativeButton(android.R.string.cancel, null)
            .setPositiveButton(R.string.share_action) { _, _ -> onConfirm() }
            .show()
        val positive = dialog.getButton(DialogInterface.BUTTON_POSITIVE)
        positive.isEnabled = false
        compared.setOnCheckedChangeListener { _, on -> positive.isEnabled = on }
    }

    private fun setBusy(busy: Boolean) {
        progress.visibility = if (busy) View.VISIBLE else View.GONE
        if (busy) errorText.visibility = View.GONE
        shareActionButton.isEnabled = !busy
    }

    private fun setShareFormEnabled(enabled: Boolean) {
        usernameInput.isEnabled = enabled
        shareActionButton.isEnabled = enabled
    }

    private fun showError(message: String) {
        errorText.text = message
        errorText.visibility = View.VISIBLE
    }

    private fun toast(message: String) =
        Toast.makeText(this, message, Toast.LENGTH_SHORT).show()

    private sealed class Outcome {
        data class Members(val members: List<ShareMember>) : Outcome()
        data class Error(val statusCode: Int, val message: String) : Outcome()
    }

    private sealed class ShareOutcome {
        data class Success(val username: String) : ShareOutcome()
        data class Failure(val message: String) : ShareOutcome()
    }

    /** Resultatet af opslaget i trin 1 — det der vises i dialogen og wrappes til. */
    private class Recipient(
        val userId: String,
        val username: String,
        val device: String,
        val publicKey: ByteArray,
        val fingerprint: String,
        val keyId: String,
    )
}
