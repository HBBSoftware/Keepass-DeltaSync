// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync

import android.app.AlertDialog
import android.widget.EditText
import android.widget.FrameLayout
import android.widget.Toast
import dk.bjoerckbraun.deltasync.ui.applySystemAndImeInsets
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.provider.OpenableColumns
import android.view.View
import android.widget.ProgressBar
import android.widget.RadioButton
import android.widget.RadioGroup
import android.widget.TextView
import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts
import androidx.lifecycle.lifecycleScope
import com.google.android.material.button.MaterialButton
import dk.bjoerckbraun.deltasync.api.ApiClient
import dk.bjoerckbraun.deltasync.api.ApiException
import dk.bjoerckbraun.deltasync.api.Database
import dk.bjoerckbraun.deltasync.persistence.DatabaseConfigStore
import dk.bjoerckbraun.deltasync.persistence.KeystoreTokenStore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/**
 * Konfigurér mapping mellem en lokal .kdbx-fil og en server-side database.
 * Tilgås fra MainActivity når brugeren er enrolled men ikke har en
 * database-config endnu.
 *
 * Flow:
 *   1. Bruger vælger .kdbx via SAF (OpenDocument-intent).
 *   2. Vi tager persistable-URI-permission så WorkManager kan tilgå
 *      filen i baggrunden.
 *   3. Bruger lister databases på serveren og picker en.
 *   4. Vi gemmer (uri, database-id, kdbx-navn) i [DatabaseConfigStore].
 */
class SetupActivity : ComponentActivity() {

    private lateinit var configStore: DatabaseConfigStore
    private lateinit var pickedFileText: TextView
    private lateinit var databaseRadios: RadioGroup
    private lateinit var saveButton: MaterialButton
    private lateinit var progress: ProgressBar
    private lateinit var errorText: TextView
    private lateinit var createHint: TextView
    private lateinit var createPrimary: MaterialButton
    private lateinit var createSecondary: MaterialButton

    private var pickedUri: Uri? = null
    private var pickedFileName: String = ""
    private var availableDatabases: List<Database> = emptyList()

    private val pickFileLauncher = registerForActivityResult(
        ActivityResultContracts.OpenDocument(),
    ) { uri: Uri? ->
        if (uri == null) return@registerForActivityResult
        // Tag persistable-permission så vi senere kan læse/skrive fra
        // WorkManager (background-tråden har ikke samme grant som
        // foreground-aktiviteten).
        val flags = Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION
        contentResolver.takePersistableUriPermission(uri, flags)
        pickedUri = uri
        pickedFileName = queryDisplayName(uri)
        pickedFileText.text = pickedFileName
        pickedFileText.visibility = View.VISIBLE
        refreshSaveButtonState()
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_setup)
        applySystemAndImeInsets()

        configStore = DatabaseConfigStore(applicationContext)
        pickedFileText = findViewById(R.id.pickedFileText)
        databaseRadios = findViewById(R.id.databaseRadios)
        saveButton = findViewById(R.id.saveConfigButton)
        progress = findViewById(R.id.setupProgress)
        errorText = findViewById(R.id.setupError)
        createHint = findViewById(R.id.createDatabaseHint)
        createPrimary = findViewById(R.id.createDatabaseButtonPrimary)
        createSecondary = findViewById(R.id.createDatabaseButton)

        findViewById<MaterialButton>(R.id.pickFileButton).setOnClickListener {
            pickFileLauncher.launch(arrayOf("*/*"))
        }

        createPrimary.setOnClickListener { promptCreateDatabase() }
        createSecondary.setOnClickListener { promptCreateDatabase() }

        findViewById<MaterialButton>(R.id.listDatabasesButton).setOnClickListener {
            listDatabases()
        }

        saveButton.setOnClickListener {
            val uri = pickedUri ?: return@setOnClickListener
            val selectedRadioId = databaseRadios.checkedRadioButtonId
            if (selectedRadioId == View.NO_ID) return@setOnClickListener
            val databaseId = availableDatabases.getOrNull(selectedRadioId)?.id ?: return@setOnClickListener
            configStore.save(uri, databaseId, pickedFileName)
            setResult(RESULT_OK)
            finish()
        }
    }

    private fun listDatabases() {
        progress.visibility = View.VISIBLE
        errorText.visibility = View.GONE
        databaseRadios.removeAllViews()

        lifecycleScope.launch {
            val outcome = withContext(Dispatchers.IO) {
                val credentials = KeystoreTokenStore(applicationContext).load()
                    ?: return@withContext DatabaseListOutcome.Failure(
                        getString(R.string.setup_error_not_enrolled))
                try {
                    val api = ApiClient(credentials.serverUrl, credentials.deviceToken)
                    DatabaseListOutcome.Success(api.listDatabases())
                } catch (e: ApiException) {
                    DatabaseListOutcome.Failure(getString(R.string.setup_error_server_format,
                        e.statusCode, e.code, e.detail))
                } catch (e: Exception) {
                    DatabaseListOutcome.Failure(e.message ?: e::class.simpleName ?: "unknown error")
                }
            }

            progress.visibility = View.GONE
            when (outcome) {
                is DatabaseListOutcome.Failure -> {
                    errorText.text = outcome.message
                    errorText.visibility = View.VISIBLE
                }

                is DatabaseListOutcome.Success -> {
                    if (outcome.databases.isEmpty()) {
                        availableDatabases = outcome.databases
                        // Ingen fejltekst her: en tom liste er ikke en fejl på
                        // en frisk server, og forklaringen står i hint'et.
                        showCreateOption(listIsEmpty = true)
                    } else {
                        renderDatabases(outcome.databases)
                        databaseRadios.check(0)
                        showCreateOption(listIsEmpty = false)
                        refreshSaveButtonState()
                    }
                }
            }
        }
    }

    /**
     * Spørg om et navn og opret databasen på serveren.
     *
     * Uden det her kunne appen kun vælge blandt databaser, som nogen havde
     * oprettet et andet sted fra — i praksis desktop-klientens `init`. En
     * bruger, der starter med telefonen, stod derfor i en blindgyde:
     * "Ingen databases på serveren", og ingen måde at lave en.
     *
     * Navnet foreslås ud fra den valgte .kdbx-fil, fordi det næsten altid er
     * det man vil kalde den, og fordi det knytter de to ting sammen visuelt.
     */
    private fun promptCreateDatabase() {
        val input = EditText(this).apply {
            hint = getString(R.string.setup_dialog_create_hint)
            setSingleLine()
            setText(suggestedDatabaseName())
            setSelectAllOnFocus(true)
        }
        val pad = (resources.displayMetrics.density * 20).toInt()
        val wrapper = FrameLayout(this).apply {
            setPadding(pad, pad / 2, pad, 0)
            addView(input)
        }

        AlertDialog.Builder(this)
            .setTitle(R.string.setup_dialog_create_title)
            .setView(wrapper)
            .setNegativeButton(android.R.string.cancel, null)
            .setPositiveButton(R.string.setup_dialog_create_ok) { _, _ ->
                val name = input.text?.toString()?.trim().orEmpty()
                if (name.isNotEmpty()) createDatabase(name)
            }
            .show()
    }

    /** Filnavnet uden sti og uden .kdbx, eller tom hvis intet er valgt endnu. */
    private fun suggestedDatabaseName(): String {
        val shown = pickedFileText.text?.toString()?.trim().orEmpty()
        return shown.substringAfterLast('/').removeSuffix(".kdbx")
    }

    private fun createDatabase(name: String) {
        progress.visibility = View.VISIBLE
        errorText.visibility = View.GONE

        lifecycleScope.launch {
            val outcome = withContext(Dispatchers.IO) {
                val credentials = KeystoreTokenStore(applicationContext).load()
                    ?: return@withContext DatabaseListOutcome.Failure(
                        getString(R.string.setup_error_not_enrolled))
                try {
                    val api = ApiClient(credentials.serverUrl, credentials.deviceToken)
                    api.createDatabase(name)
                    DatabaseListOutcome.Success(api.listDatabases())
                } catch (e: ApiException) {
                    DatabaseListOutcome.Failure(getString(R.string.setup_error_server_format,
                        e.statusCode, e.code, e.detail))
                } catch (e: Exception) {
                    DatabaseListOutcome.Failure(e.message ?: e::class.simpleName ?: "unknown error")
                }
            }

            progress.visibility = View.GONE
            when (outcome) {
                is DatabaseListOutcome.Failure -> {
                    errorText.text = outcome.message
                    errorText.visibility = View.VISIBLE
                }

                is DatabaseListOutcome.Success -> {
                    // Vis listen med det samme og markér den nye, så trin 2 er
                    // gjort færdigt i samme bevægelse i stedet for at bede
                    // brugeren trykke "List databases" bagefter.
                    renderDatabases(outcome.databases)
                    showCreateOption(listIsEmpty = false)
                    val index = outcome.databases.indexOfFirst { it.name == name }
                    if (index >= 0) databaseRadios.check(index)
                    refreshSaveButtonState()
                    Toast.makeText(this@SetupActivity,
                        getString(R.string.setup_created_format, name),
                        Toast.LENGTH_LONG).show()
                }
            }
        }
    }

    /**
     * Vis opret-muligheden i den form situationen kalder på.
     *
     * Er der ingen databaser, er oprettelse den eneste vej videre, og så skal
     * knappen være fremhævet og forklaret. Er der databaser, er den en
     * sidevej — så står den diskret, så den ikke konkurrerer med det valg,
     * brugeren er ved at træffe.
     */
    private fun showCreateOption(listIsEmpty: Boolean) {
        createHint.visibility = if (listIsEmpty) View.VISIBLE else View.GONE
        createPrimary.visibility = if (listIsEmpty) View.VISIBLE else View.GONE
        createSecondary.visibility = if (listIsEmpty) View.GONE else View.VISIBLE
    }

    /** Byg radiolisten. Delt af listDatabases og createDatabase. */
    private fun renderDatabases(databases: List<Database>) {
        availableDatabases = databases
        databaseRadios.removeAllViews()
        databases.forEachIndexed { index, db ->
            val radio = RadioButton(this@SetupActivity).apply {
                id = index
                text = getString(R.string.setup_database_radio_format,
                    db.name, db.role, db.id.take(8))
                textSize = 14f
            }
            databaseRadios.addView(radio)
        }
    }

    private fun refreshSaveButtonState() {
        saveButton.isEnabled = pickedUri != null && databaseRadios.childCount > 0
    }

    private fun queryDisplayName(uri: Uri): String {
        contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { cursor ->
            if (cursor.moveToFirst()) {
                val idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                if (idx >= 0) return cursor.getString(idx)
            }
        }
        return uri.lastPathSegment.orEmpty()
    }

    private sealed class DatabaseListOutcome {
        data class Success(val databases: List<Database>) : DatabaseListOutcome()
        data class Failure(val message: String) : DatabaseListOutcome()
    }
}
