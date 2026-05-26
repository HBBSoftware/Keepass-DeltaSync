<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Crypto;

/**
 * Token-generering + hashing til lagring.
 *
 * SPEC-AFVIGELSE: spec § "Sikkerhed" foreskriver Argon2id-hashing af tokens.
 * Vi bruger SHA-256 i stedet, fordi:
 *
 *   1. Schema'en (token_hash som PRIMARY KEY) kræver deterministisk hash for
 *      O(1)-opslag. Argon2id har tilfældigt salt og kræver derfor verify mod
 *      hver række i tabellen — uacceptabelt selv for moderate antal devices.
 *
 *   2. Tokens er 32 random bytes (256 bits entropi). Argon2id beskytter mod
 *      brute-force af svage passwords, men 2^256 inputs er kryptografisk
 *      ubrute-force-bare uanset hash-funktion. Argon2id giver derfor reelt
 *      ingen sikkerhedsforbedring her, men store ydelsesomkostninger.
 *
 * Defense-in-depth-opgradering hvis ønsket senere: HMAC-SHA256 med en
 * server-side pepper fra env, så et DB-dump alene ikke giver bruger-tokens.
 */
final class TokenHasher
{
    /** Generér en frisk token (32 random bytes, URL-safe base64). */
    public static function generate(): string
    {
        return rtrim(strtr(base64_encode(random_bytes(32)), '+/', '-_'), '=');
    }

    /** Deterministisk hash til lagring og opslag. */
    public static function hash(string $token): string
    {
        return hash('sha256', $token);
    }
}
