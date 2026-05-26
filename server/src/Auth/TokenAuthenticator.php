<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Auth;

use KeePassDeltaSync\Crypto\TokenHasher;
use PDO;

/**
 * Validerer en bearer-token mod databasen og returnerer en AuthContext.
 *
 * Hver route i Router har en required token-type (admin / enrollment / device).
 * authenticate() kaldes med den krævede type — token'en valideres KUN mod den
 * relevante tabel, så et admin-token kan ikke "smutte ind" på et bruger-endpoint
 * (eller omvendt).
 *
 * Side effects:
 *   - admin_tokens.last_used opdateres ved hver vellykket admin-validering.
 *   - devices.last_seen opdateres ved hver vellykket device-validering.
 *   - enrollment_tokens markeres IKKE som brugt her — det gøres af enrollment-
 *     controlleren først når enheden er færdig-oprettet (transaktion).
 */
final class TokenAuthenticator
{
    public function __construct(private readonly PDO $pdo) {}

    public function authenticate(string $bearer, TokenType $required): AuthContext
    {
        if ($bearer === '') {
            throw new AuthenticationException('missing token');
        }

        $hash = TokenHasher::hash($bearer);

        $ctx = match ($required) {
            TokenType::Admin      => $this->lookupAdmin($hash),
            TokenType::Enrollment => $this->lookupEnrollment($hash),
            TokenType::Device     => $this->lookupDevice($hash),
        };

        if ($ctx === null) {
            throw new AuthenticationException('invalid token');
        }
        return $ctx;
    }

    private function lookupAdmin(string $hash): ?AuthContext
    {
        $stmt = $this->pdo->prepare(
            'SELECT token_hash FROM admin_tokens WHERE token_hash = :h'
        );
        $stmt->execute(['h' => $hash]);
        if (!$stmt->fetch()) {
            return null;
        }

        $upd = $this->pdo->prepare(
            'UPDATE admin_tokens SET last_used = now() WHERE token_hash = :h'
        );
        $upd->execute(['h' => $hash]);

        return new AuthContext(TokenType::Admin);
    }

    private function lookupEnrollment(string $hash): ?AuthContext
    {
        $stmt = $this->pdo->prepare(
            'SELECT et.user_id
               FROM enrollment_tokens et
               JOIN users u ON u.id = et.user_id AND u.disabled = false
              WHERE et.token_hash = :h
                AND et.used       = false
                AND et.expires_at > now()'
        );
        $stmt->execute(['h' => $hash]);
        $row = $stmt->fetch();
        if (!$row) {
            return null;
        }

        return new AuthContext(
            type:                TokenType::Enrollment,
            userId:              $row['user_id'],
            enrollmentTokenHash: $hash,
        );
    }

    private function lookupDevice(string $hash): ?AuthContext
    {
        $stmt = $this->pdo->prepare(
            'SELECT d.id AS device_id, d.user_id
               FROM devices d
               JOIN users u ON u.id = d.user_id AND u.disabled = false
              WHERE d.token_hash = :h'
        );
        $stmt->execute(['h' => $hash]);
        $row = $stmt->fetch();
        if (!$row) {
            return null;
        }

        $upd = $this->pdo->prepare(
            'UPDATE devices SET last_seen = now() WHERE id = :id'
        );
        $upd->execute(['id' => $row['device_id']]);

        return new AuthContext(
            type:     TokenType::Device,
            userId:   $row['user_id'],
            deviceId: $row['device_id'],
        );
    }
}
