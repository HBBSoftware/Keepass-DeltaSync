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
        public readonly string $basePath,
        public readonly int    $adminSessionTtlHours,
        public readonly int    $rateLimitAuthPerMinute,
        public readonly int    $argon2MemoryCost,
        public readonly int    $argon2TimeCost,
        public readonly int    $argon2Threads,
        /** @var list<string> IP'er/CIDR'er hvis X-Forwarded-*-headere vi stoler på. */
        public readonly array  $trustedProxies,
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
            basePath:                self::normalizeBasePath(self::env('APP_BASE_PATH', '')),
            adminSessionTtlHours:    (int) self::env('ADMIN_SESSION_TTL_HOURS', '8'),
            rateLimitAuthPerMinute:  (int) self::env('RATE_LIMIT_AUTH_PER_MINUTE', '10'),
            argon2MemoryCost:        (int) self::env('ARGON2_MEMORY_COST', '65536'),
            argon2TimeCost:          (int) self::env('ARGON2_TIME_COST', '4'),
            argon2Threads:           (int) self::env('ARGON2_THREADS', '1'),
            trustedProxies:          self::parseList(self::env('TRUSTED_PROXIES', '')),
        );
    }

    /**
     * normalizeBasePath sikrer at base-path er enten tom string (= serveret
     * på server-roden) eller "/prefix" (leading slash, ingen trailing slash).
     * Brugeren kan angive med eller uden slashes; vi normaliserer.
     */
    /**
     * Argon2id-parametre i det format password_hash() forventer.
     *
     * @return array{memory_cost:int, time_cost:int, threads:int}
     */
    public function argon2Options(): array
    {
        return [
            'memory_cost' => $this->argon2MemoryCost,
            'time_cost'   => $this->argon2TimeCost,
            'threads'     => $this->argon2Threads,
        ];
    }

    /**
     * Komma-separeret liste → array uden tomme elementer.
     *
     * @return list<string>
     */
    private static function parseList(string $raw): array
    {
        $parts = array_map('trim', explode(',', $raw));
        return array_values(array_filter($parts, static fn(string $p): bool => $p !== ''));
    }

    private static function normalizeBasePath(string $raw): string
    {
        $raw = trim($raw);
        if ($raw === '' || $raw === '/') {
            return '';
        }
        $raw = '/' . ltrim($raw, '/');
        return rtrim($raw, '/');
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
