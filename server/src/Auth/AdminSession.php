<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Auth;

use KeePassDeltaSync\Crypto\TokenHasher;
use PDO;

/**
 * Browser-sessioner til admin-panelet.
 *
 * Session-tokenet er 32 tilfældige bytes — samme styrke som et admin-token —
 * og lagres kun som SHA-256-hash. Det lever i en HttpOnly-cookie, så
 * JavaScript aldrig kan læse det; det er hele grunden til at panelet ikke
 * længere holder legitimationen i sessionStorage.
 *
 * Levetiden er glidende: hvert opslag skubber expires_at frem, så en
 * arbejdsdag i panelet kræver ét login, mens en glemt fane udløber af sig selv.
 */
final class AdminSession
{
    public const COOKIE_NAME = 'deltasync_admin_session';

    /** @return string det rå token (vises kun her, gemmes hashet) */
    public static function create(PDO $pdo, int $ttlHours): string
    {
        self::gc($pdo);

        $token = TokenHasher::generate();
        $stmt  = $pdo->prepare(
            "INSERT INTO admin_sessions (token_hash, expires_at)
             VALUES (:h, now() + (:ttl || ' hours')::interval)"
        );
        $stmt->execute(['h' => TokenHasher::hash($token), 'ttl' => (string) $ttlHours]);

        return $token;
    }

    /**
     * Slå en session op og forny den. Returnerer false for ukendte og
     * udløbne tokens — udløb afgøres i SQL, så en stoppet cleanup-opgave
     * ikke kan holde en session kunstigt i live.
     */
    public static function validate(PDO $pdo, string $token, int $ttlHours): bool
    {
        if ($token === '') {
            return false;
        }
        $hash = TokenHasher::hash($token);

        $stmt = $pdo->prepare(
            'SELECT 1 FROM admin_sessions WHERE token_hash = :h AND expires_at > now()'
        );
        $stmt->execute(['h' => $hash]);
        if ($stmt->fetchColumn() === false) {
            return false;
        }

        $upd = $pdo->prepare(
            "UPDATE admin_sessions
                SET last_used  = now(),
                    expires_at = now() + (:ttl || ' hours')::interval
              WHERE token_hash = :h"
        );
        $upd->execute(['h' => $hash, 'ttl' => (string) $ttlHours]);

        return true;
    }

    public static function destroy(PDO $pdo, string $token): void
    {
        if ($token === '') {
            return;
        }
        $stmt = $pdo->prepare('DELETE FROM admin_sessions WHERE token_hash = :h');
        $stmt->execute(['h' => TokenHasher::hash($token)]);
    }

    /** Slet alle sessioner — bruges når adgangskoden ændres. */
    public static function destroyAll(PDO $pdo): void
    {
        $pdo->exec('DELETE FROM admin_sessions');
    }

    private static function gc(PDO $pdo): void
    {
        $pdo->exec('DELETE FROM admin_sessions WHERE expires_at <= now()');
    }
}
