<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Auth;

use KeePassDeltaSync\Config;
use PDO;

/**
 * Den ene administrator-konto (brugernavn + Argon2id-hashet adgangskode).
 *
 * Modsat tokens er en adgangskode gætbar, så her er en langsom, saltet hash
 * det rigtige valg. Se src/Crypto/TokenHasher.php for hvorfor tokens omvendt
 * bruger SHA-256 — de to valg er ikke i modstrid, de svarer på forskellige
 * trusler.
 *
 * Skemaet tillader kun én række (CHECK på singleton-nøglen), så alle metoder
 * her arbejder på "kontoen" i bestemt form.
 */
final class AdminAccount
{
    public static function exists(PDO $pdo): bool
    {
        return (bool) $pdo->query('SELECT 1 FROM admin_account')->fetchColumn();
    }

    public static function username(PDO $pdo): ?string
    {
        $value = $pdo->query('SELECT username FROM admin_account')->fetchColumn();
        return is_string($value) ? $value : null;
    }

    /**
     * Opret eller opdatér kontoen. Idempotent: kaldes ved hver opstart når
     * ADMIN_USERNAME/ADMIN_PASSWORD er sat, og skriver kun hvis noget faktisk
     * er ændret — ellers ville hver genstart producere en ny hash og en ny
     * updated_at uden grund.
     *
     * @return bool true hvis rækken blev skrevet
     */
    public static function set(PDO $pdo, string $username, string $password, Config $config): bool
    {
        $row = $pdo->query('SELECT username, password_hash FROM admin_account')->fetch();

        if ($row !== false
            && $row['username'] === $username
            && password_verify($password, (string) $row['password_hash'])
            && !password_needs_rehash((string) $row['password_hash'], PASSWORD_ARGON2ID, $config->argon2Options())
        ) {
            return false;
        }

        $hash = password_hash($password, PASSWORD_ARGON2ID, $config->argon2Options());
        if (!is_string($hash)) {
            throw new \RuntimeException('kunne ikke hashe admin-adgangskoden');
        }

        $stmt = $pdo->prepare(
            'INSERT INTO admin_account (singleton, username, password_hash)
             VALUES (TRUE, :u, :h)
             ON CONFLICT (singleton) DO UPDATE
                SET username = EXCLUDED.username,
                    password_hash = EXCLUDED.password_hash,
                    updated_at = now()'
        );
        $stmt->execute(['u' => $username, 'h' => $hash]);
        return true;
    }

    /**
     * Verificér login. Kører altid en hash-beregning, også når kontoen ikke
     * findes eller brugernavnet er forkert, så svartiden ikke røber hvilken
     * del der fejlede.
     */
    public static function verify(PDO $pdo, string $username, string $password, Config $config): bool
    {
        $row = $pdo->query('SELECT username, password_hash FROM admin_account')->fetch();

        $storedHash = is_array($row) ? (string) $row['password_hash'] : null;
        $storedUser = is_array($row) ? (string) $row['username'] : null;

        // Dummy-hash når der ikke er en konto: password_verify skal stadig
        // bruge sammenlignelig tid. Gyldig Argon2id-streng, umulig at ramme.
        $candidate = $storedHash ?? '$argon2id$v=19$m=65536,t=4,p=1$'
            . 'ZGVsdGFzeW5jZHVtbXlzYWx0$3Fh9nJ0PPr4RoW4tSGjw0ADRJd7eGpB0M6h2vJ2mE0A';

        $passwordOk = password_verify($password, $candidate);
        $userOk     = $storedUser !== null && hash_equals($storedUser, $username);

        if ($storedHash === null || !$passwordOk || !$userOk) {
            return false;
        }

        if (password_needs_rehash($storedHash, PASSWORD_ARGON2ID, $config->argon2Options())) {
            $rehashed = password_hash($password, PASSWORD_ARGON2ID, $config->argon2Options());
            if (is_string($rehashed)) {
                $upd = $pdo->prepare('UPDATE admin_account SET password_hash = :h, updated_at = now()');
                $upd->execute(['h' => $rehashed]);
            }
        }

        $pdo->exec('UPDATE admin_account SET last_login = now()');
        return true;
    }
}
