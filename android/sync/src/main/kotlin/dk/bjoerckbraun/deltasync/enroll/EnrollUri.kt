// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.enroll

import java.net.URI
import java.net.URLDecoder

/**
 * De felter en enrollment-QR bærer: alt hvad [EnrollActivity]-flowet skal bruge
 * for at kunne enrolle uden at brugeren taster noget. Server-URL og token er
 * påkrævet; enheds-navn er valgfrit forslag.
 */
data class EnrollUri(
    val server: String,
    val token: String,
    val deviceName: String? = null,
)

/**
 * Parser den `deltasync://enroll`-URI som admin-panelet koder ind i en QR:
 *
 *   deltasync://enroll?server=<url-encoded>&token=<url-encoded>&name=<optional>
 *
 * Formatet er bevidst minimalt og læsbart så en QR kan genereres af hvad som
 * helst (admin-panelet, en offline generator) og verificeres i hånden. Både
 * server og token er URL-encodede (`encodeURIComponent`) — tokens er base64 og
 * kan indeholde `+`/`/`/`=` som ellers ville kollidere med query-syntaksen.
 *
 * Returnerer `null` for alt der ikke er en gyldig enroll-URI (forkert scheme,
 * manglende felter, server-URL uden http(s)-scheme). Kaldere kan så falde
 * tilbage til manuel indtastning uden at skulle håndtere exceptions.
 */
object EnrollUriParser {

    const val SCHEME: String = "deltasync"
    const val HOST: String = "enroll"

    fun parse(raw: String): EnrollUri? {
        val text = raw.trim()
        if (text.isEmpty()) return null

        val uri = runCatching { URI(text) }.getOrNull() ?: return null
        if (!SCHEME.equals(uri.scheme, ignoreCase = true)) return null
        // Authority bærer "enroll" (deltasync://enroll?...). URI ser det som host.
        val host = uri.host ?: uri.authority
        if (!HOST.equals(host, ignoreCase = true)) return null

        val params = parseQuery(uri.rawQuery ?: return null)
        val server = params["server"]?.trim().orEmpty()
        val token = params["token"]?.trim().orEmpty()
        if (token.isEmpty()) return null
        if (!server.startsWith("http://") && !server.startsWith("https://")) return null

        val name = params["name"]?.trim()?.takeIf { it.isNotEmpty() }
        return EnrollUri(server = server, token = token, deviceName = name)
    }

    /**
     * Dekoder en `a=b&c=d`-query til et map. Bruger den sidste værdi hvis en
     * nøgle optræder flere gange. `+` behandles som mellemrum jf.
     * application/x-www-form-urlencoded, matchende afsenderens
     * `encodeURIComponent` (der aldrig selv udsender bart `+`).
     */
    private fun parseQuery(query: String): Map<String, String> {
        if (query.isEmpty()) return emptyMap()
        val out = LinkedHashMap<String, String>()
        for (pair in query.split('&')) {
            if (pair.isEmpty()) continue
            val eq = pair.indexOf('=')
            val key = if (eq < 0) pair else pair.substring(0, eq)
            val value = if (eq < 0) "" else pair.substring(eq + 1)
            out[decode(key)] = decode(value)
        }
        return out
    }

    private fun decode(s: String): String =
        runCatching { URLDecoder.decode(s, Charsets.UTF_8.name()) }.getOrDefault(s)
}
