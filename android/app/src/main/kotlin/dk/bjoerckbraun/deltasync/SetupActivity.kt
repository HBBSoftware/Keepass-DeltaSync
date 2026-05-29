// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync

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

        configStore = DatabaseConfigStore(applicationContext)
        pickedFileText = findViewById(R.id.pickedFileText)
        databaseRadios = findViewById(R.id.databaseRadios)
        saveButton = findViewById(R.id.saveConfigButton)
        progress = findViewById(R.id.setupProgress)
        errorText = findViewById(R.id.setupError)

        findViewById<MaterialButton>(R.id.pickFileButton).setOnClickListener {
            pickFileLauncher.launch(arrayOf("*/*"))
        }

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
                    ?: return@withContext DatabaseListOutcome.Failure("Not enrolled.")
                try {
                    val api = ApiClient(credentials.serverUrl, credentials.deviceToken)
                    DatabaseListOutcome.Success(api.listDatabases())
                } catch (e: ApiException) {
                    DatabaseListOutcome.Failure("Server: ${e.statusCode} ${e.code} ${e.detail}")
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
                    availableDatabases = outcome.databases
                    if (outcome.databases.isEmpty()) {
                        errorText.text = "Ingen databases på serveren. Opret én via desktop-klienten først."
                        errorText.visibility = View.VISIBLE
                    } else {
                        outcome.databases.forEachIndexed { index, db ->
                            val radio = RadioButton(this@SetupActivity).apply {
                                id = index
                                text = "${db.name} (${db.role}) — ${db.id.take(8)}…"
                                textSize = 14f
                            }
                            databaseRadios.addView(radio)
                        }
                        databaseRadios.check(0)
                        refreshSaveButtonState()
                    }
                }
            }
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
