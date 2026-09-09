// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.ui

import android.app.Activity
import android.view.View
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat

/**
 * Giver indholdet plads til systemlinjerne og til tastaturet.
 *
 * Fra targetSdk 35 tegner Android 15 apps edge-to-edge og honorerer ikke
 * længere windowSoftInputMode="adjustResize" som før. Uden det her lægger
 * tastaturet sig oven på indtastningsfeltet i bunden af skærmen: man kan
 * skrive, men ikke se hvad man skriver. Alle fire skærme er ScrollViews, så
 * så snart bundpolstringen er rigtig, kan indholdet rulles fri af tastaturet.
 *
 * Bundpolstringen er den største af navigationslinjen og tastaturet frem for
 * summen — når tastaturet er fremme, ligger det oven på navigationslinjen, og
 * lagt sammen ville der komme et tomt bånd under feltet.
 */
fun Activity.applySystemAndImeInsets() {
    val content = findViewById<View>(android.R.id.content)

    ViewCompat.setOnApplyWindowInsetsListener(content) { view, insets ->
        val bars = insets.getInsets(WindowInsetsCompat.Type.systemBars())
        val ime = insets.getInsets(WindowInsetsCompat.Type.ime())

        view.setPadding(bars.left, bars.top, bars.right, maxOf(bars.bottom, ime.bottom))
        insets
    }
}
