<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync;

/**
 * Read-only konfigurationsobjekt. Læses fra .env + environment.
 * Skelet: kun de vigtigste felter er typed; flere kommer ind efterhånden
 * som controller/repo-kode tager dem i brug.
 */
final class Config
{
    public function __construct(
        public readonly string $databaseDsn,
        public readonly string $databaseUser,
        public readonly string $databasePassword,
        public readonly string $logLevel,
        public readonly int    $auditRetentionDays,
        public readonly int    $enrollmentTokenTtlHours,
    ) {}

    public static function loadFromEnv(string $rootDir): self
    {
        self::loadDotEnv($rootDir . '/.env');

        return new self(
            databaseDsn:             self::env('DATABASE_URL'),
            databaseUser:            self::env('DATABASE_USER'),
            databasePassword:        self::env('DATABASE_PASSWORD'),
            logLevel:                self::env('LOG_LEVEL', 'INFO'),
            auditRetentionDays:      (int) self::env('AUDIT_RETENTION_DAYS', '30'),
            enrollmentTokenTtlHours: (int) self::env('ENROLLMENT_TOKEN_TTL_HOURS', '24'),
        );
    }

    private static function loadDotEnv(string $path): void
    {
        if (!is_readable($path)) {
            return;
        }
        foreach (file($path, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES) as $line) {
            $line = trim($line);
            if ($line === '' || str_starts_with($line, '#')) {
                continue;
            }
            [$key, $value] = array_pad(explode('=', $line, 2), 2, '');
            $key = trim($key);
            $value = trim($value);
            if ($key !== '' && getenv($key) === false) {
                putenv("$key=$value");
                $_ENV[$key] = $value;
            }
        }
    }

    private static function env(string $key, ?string $default = null): string
    {
        $value = getenv($key);
        if ($value === false || $value === '') {
            if ($default !== null) {
                return $default;
            }
            throw new \RuntimeException("Missing required env var: $key");
        }
        return $value;
    }
}
