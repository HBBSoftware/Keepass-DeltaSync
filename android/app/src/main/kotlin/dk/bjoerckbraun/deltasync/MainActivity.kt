// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync

import android.os.Bundle
import android.widget.TextView
import androidx.activity.ComponentActivity

/**
 * Minimal entry-Activity for v0.1. Vil senere blive erstattet af en
 * Compose-baseret UI med enroll-flow, database-liste og sync-status.
 *
 * For nu viser den blot at app'en er installeret og at gomobile-bound
 * krypto-laget kan loades — det validerer at JNI-libsene faktisk
 * loader på enheden uden at vi behøver UI.
 */
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val tv = TextView(this).apply {
            text = buildStatusText()
            textSize = 16f
            setPadding(48, 96, 48, 48)
        }
        setContentView(tv)
    }

    private fun buildStatusText(): String {
        val sb = StringBuilder()
        sb.appendLine("DeltaSync v0.1.0")
        sb.appendLine()
        sb.appendLine("Status: scaffold — not yet implemented.")
        sb.appendLine()
        sb.appendLine("Schema version: ")
        sb.append(runCatching {
            mobile.Mobile.SchemaVersion.toString()
        }.getOrElse { "could not load JNI: ${it.message}" })
        sb.appendLine()
        return sb.toString()
    }
}
