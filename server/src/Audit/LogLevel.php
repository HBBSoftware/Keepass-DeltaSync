<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Audit;

/**
 * Audit-log-niveau. INFO i drift, DEBUG i udvikling. Styres via env-var
 * `LOG_LEVEL` (Config::loadFromEnv) — værdier sammenlignes case-insensitive.
 */
enum LogLevel: string
{
    case Info  = 'INFO';
    case Debug = 'DEBUG';

    public static function fromConfig(string $raw): self
    {
        return match (strtoupper(trim($raw))) {
            'INFO'  => self::Info,
            'DEBUG' => self::Debug,
            default => throw new \InvalidArgumentException("invalid LOG_LEVEL: $raw"),
        };
    }
}
