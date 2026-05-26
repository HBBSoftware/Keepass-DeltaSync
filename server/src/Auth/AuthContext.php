<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Auth;

/**
 * Resultat af en vellykket auth-validering.
 *
 *   - Admin-tokens:      $type = Admin; ingen userId/deviceId.
 *   - Enrollment-tokens: $type = Enrollment; $userId sat, $deviceId null,
 *                         $enrollmentTokenHash bruges af enrollment-flowet til
 *                         at markere token'en som brugt.
 *   - Device-tokens:     $type = Device; $userId + $deviceId sat.
 */
final class AuthContext
{
    public function __construct(
        public readonly TokenType $type,
        public readonly ?string   $userId              = null,
        public readonly ?string   $deviceId            = null,
        public readonly ?string   $enrollmentTokenHash = null,
    ) {}
}
