<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Audit;

use PDO;

/**
 * Periodisk oprydning af `audit_log` og udløbne/brugte `enrollment_tokens`.
 *
 * To call sites:
 *
 *   1. **App::handle() ved opstart** kalder `runIfDue()`. Throttled til
 *      1×/time via `system_state.last_cleanup_at`. Lock'en sørger for at
 *      kun én PHP-worker rydder op selv hvis flere starter samtidigt.
 *
 *   2. **POST /admin/log/cleanup** kalder `runForced()`. Springer throttle
 *      men holder stadig lock'en — to admins kan ikke trigge oprydning
 *      samtidigt og udløse race.
 *
 * Lock'en er pg_try_advisory_lock — non-blocking. Hvis en anden process har
 * den, returnerer `runIfDue()` null og `runForced()` kaster en RuntimeException
 * som controlleren mapper til 409.
 *
 * Bemærk: oprydning kan ikke køre i en eksisterende transaktion fordi
 * advisory locks frigives ved transaction commit/rollback. Vi bruger derfor
 * auto-commit mode (default for PDO udenfor beginTransaction).
 */
final class Cleanup
{
    /** Vilkårligt unikt heltal — pg_advisory_lock kræver et lock-ID i denne range. */
    private const int ADVISORY_LOCK_KEY = 42;

    /** Hvor ofte cleanup må køre fra startup-trigger (sekunder). */
    private const int THROTTLE_SECONDS = 3600;

    /**
     * Best-effort cleanup. Returnerer null hvis throttled eller hvis en anden
     * worker har lock'en.
     *
     * @return array{audit_rows_deleted:int, enrollment_rows_deleted:int}|null
     */
    public static function runIfDue(PDO $pdo, int $retentionDays): ?array
    {
        if (!self::tryLock($pdo)) {
            return null;
        }
        try {
            if (self::isThrottled($pdo)) {
                return null;
            }
            return self::doCleanup($pdo, $retentionDays);
        } finally {
            self::unlock($pdo);
        }
    }

    /**
     * Forced cleanup — springer throttle. Holder stadig lock'en.
     *
     * @return array{audit_rows_deleted:int, enrollment_rows_deleted:int}
     * @throws \RuntimeException hvis lock'en er optaget af anden process
     */
    public static function runForced(PDO $pdo, int $retentionDays): array
    {
        if (!self::tryLock($pdo)) {
            throw new \RuntimeException('cleanup already in progress');
        }
        try {
            return self::doCleanup($pdo, $retentionDays);
        } finally {
            self::unlock($pdo);
        }
    }

    private static function tryLock(PDO $pdo): bool
    {
        $stmt = $pdo->query('SELECT pg_try_advisory_lock(' . self::ADVISORY_LOCK_KEY . ')');
        return (bool) $stmt->fetchColumn();
    }

    private static function unlock(PDO $pdo): void
    {
        $pdo->query('SELECT pg_advisory_unlock(' . self::ADVISORY_LOCK_KEY . ')');
    }

    private static function isThrottled(PDO $pdo): bool
    {
        $stmt = $pdo->query("SELECT value FROM system_state WHERE key = 'last_cleanup_at'");
        $lastRun = $stmt->fetchColumn();
        if ($lastRun === false) {
            return false;
        }
        try {
            $last = new \DateTimeImmutable((string) $lastRun);
        } catch (\Exception) {
            return false;  // corrupt data — lad cleanup køre og overskriv
        }
        $threshold = new \DateTimeImmutable('-' . self::THROTTLE_SECONDS . ' seconds');
        return $last > $threshold;
    }

    /**
     * @return array{audit_rows_deleted:int, enrollment_rows_deleted:int}
     */
    private static function doCleanup(PDO $pdo, int $retentionDays): array
    {
        $cutoff = (new \DateTimeImmutable('-' . $retentionDays . ' days'))
            ->format('Y-m-d H:i:sP');

        $stmt = $pdo->prepare('DELETE FROM audit_log WHERE occurred_at < :cutoff');
        $stmt->execute(['cutoff' => $cutoff]);
        $auditRows = $stmt->rowCount();

        $enrolStmt = $pdo->query(
            'DELETE FROM enrollment_tokens WHERE expires_at < now() OR used = true'
        );
        $enrolRows = $enrolStmt->rowCount();

        // Marker tidspunktet — INSERT eller UPDATE.
        $pdo->exec(
            "INSERT INTO system_state (key, value, updated_at)
             VALUES ('last_cleanup_at', now()::text, now())
             ON CONFLICT (key) DO UPDATE
                SET value = excluded.value, updated_at = now()"
        );

        return [
            'audit_rows_deleted'      => $auditRows,
            'enrollment_rows_deleted' => $enrolRows,
        ];
    }
}
