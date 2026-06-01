// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync

/**
 * Holder master-passwordet i hukommelsen så længe app-processen kører — ikke
 * længere. Bruges når brugeren krydser "Husk password"-checkboxen i sync-
 * dialogen, så efterfølgende syncs i samme session ikke prompter igen.
 *
 * Bevidst IKKE persisteret: hverken til disk, DataStore eller Keystore. Når
 * Android dræber processen (eller brugeren "Force stopper" app'en), er
 * passwordet væk. Det ryddes også eksplicit ved sync-fejl (forkert password
 * skal ikke blive hængende) og ved "Glem enheds-credentials".
 */
object SessionPassphrase {
    @Volatile
    private var cached: String? = null

    val isRemembered: Boolean get() = cached != null

    fun get(): String? = cached

    fun remember(passphrase: String) {
        cached = passphrase
    }

    fun clear() {
        cached = null
    }
}
