<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Auth;

use PDO;

/**
 * Rate limiting af login-forsøg, pr. IP pr. minut.
 *
 * Nødvendig først nu. Et admin-token er 32 tilfældige bytes og kan ikke
 * brute-forces uanset hvor mange forsøg nogen får lov til; en adgangskode
 * kan. Tælleren lever i databasen, ikke i hukommelsen, fordi Apache kører
 * flere workers — en in-process tæller ville give hver worker sin egen kvote.
 */
final class RateLimiter
{
    /** IP'en er ukendt (ingen REMOTE_ADDR). Alle sådanne deler én kvote. */
    private const UNKNOWN_IP = 'unknown';

    public static function tooManyAttempts(PDO $pdo, ?string $ip, int $perMinute): bool
    {
        if ($perMinute <= 0) {
            return false; // 0 eller negativ = slået fra
        }

        $stmt = $pdo->prepare(
            "SELECT attempts FROM auth_attempts
              WHERE ip = :ip AND window_start = date_trunc('minute', now())"
        );
        $stmt->execute(['ip' => $ip ?? self::UNKNOWN_IP]);
        $attempts = $stmt->fetchColumn();

        return is_numeric($attempts) && (int) $attempts >= $perMinute;
    }

    /** Tæl et mislykket forsøg. Kun fejl tæller — et gyldigt login er gratis. */
    public static function record(PDO $pdo, ?string $ip): void
    {
        $stmt = $pdo->prepare(
            "INSERT INTO auth_attempts (ip, window_start, attempts)
             VALUES (:ip, date_trunc('minute', now()), 1)
             ON CONFLICT (ip, window_start)
             DO UPDATE SET attempts = auth_attempts.attempts + 1"
        );
        $stmt->execute(['ip' => $ip ?? self::UNKNOWN_IP]);

        // Rækkerne er kun interessante i det minut de gælder. Ryd op her frem
        // for i en cron, så tabellen ikke vokser i en installation ingen rører.
        $pdo->exec("DELETE FROM auth_attempts WHERE window_start < now() - interval '1 hour'");
    }

    /** Nulstil efter et vellykket login, så en tastefejl ikke straffer videre. */
    public static function clear(PDO $pdo, ?string $ip): void
    {
        $stmt = $pdo->prepare('DELETE FROM auth_attempts WHERE ip = :ip');
        $stmt->execute(['ip' => $ip ?? self::UNKNOWN_IP]);
    }
}
