// SPDX-License-Identifier: GPL-3.0-or-later
package dk.bjoerckbraun.deltasync.enroll

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class EnrollUriParserTest {

    @Test
    fun parsesServerAndToken() {
        val uri = "deltasync://enroll?server=https%3A%2F%2Fdeltasync.example.dk&token=abc123"
        val out = EnrollUriParser.parse(uri)
        assertEquals("https://deltasync.example.dk", out?.server)
        assertEquals("abc123", out?.token)
        assertNull(out?.deviceName)
    }

    @Test
    fun parsesOptionalDeviceName() {
        val uri = "deltasync://enroll?server=https%3A%2F%2Fhost&token=t&name=Hans%27%20Pixel"
        val out = EnrollUriParser.parse(uri)
        assertEquals("Hans' Pixel", out?.deviceName)
    }

    @Test
    fun preservesBase64TokenSpecialChars() {
        // Standard-base64 tokens indeholder +, / og = som skal overleve
        // encode→decode-runden urørte (encodeURIComponent → %2B osv.).
        val token = "aB+cD/eF=="
        val enc = "aB%2BcD%2FeF%3D%3D"
        val out = EnrollUriParser.parse("deltasync://enroll?server=https%3A%2F%2Fh&token=$enc")
        assertEquals(token, out?.token)
    }

    @Test
    fun treatsPlusAsSpaceInNameOnlyViaEncoding() {
        // Et bart '+' i query dekodes som mellemrum (form-urlencoded), men et
        // korrekt %2B-encodet '+' bevares — vi verificerer token-stien ovenfor;
        // her at bart '+' i navn bliver mellemrum.
        val out = EnrollUriParser.parse("deltasync://enroll?server=https%3A%2F%2Fh&token=t&name=a+b")
        assertEquals("a b", out?.deviceName)
    }

    @Test
    fun rejectsWrongScheme() {
        assertNull(EnrollUriParser.parse("https://enroll?server=https%3A%2F%2Fh&token=t"))
        assertNull(EnrollUriParser.parse("otpauth://enroll?token=t"))
    }

    @Test
    fun rejectsWrongHost() {
        assertNull(EnrollUriParser.parse("deltasync://share?server=https%3A%2F%2Fh&token=t"))
    }

    @Test
    fun rejectsMissingToken() {
        assertNull(EnrollUriParser.parse("deltasync://enroll?server=https%3A%2F%2Fh"))
    }

    @Test
    fun rejectsServerWithoutHttpScheme() {
        assertNull(EnrollUriParser.parse("deltasync://enroll?server=ftp%3A%2F%2Fh&token=t"))
        assertNull(EnrollUriParser.parse("deltasync://enroll?server=deltasync.example.dk&token=t"))
    }

    @Test
    fun rejectsGarbage() {
        assertNull(EnrollUriParser.parse(""))
        assertNull(EnrollUriParser.parse("   "))
        assertNull(EnrollUriParser.parse("just some text"))
        assertNull(EnrollUriParser.parse("https://deltasync.example.dk"))
    }
}
