<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Controllers;

use KeePassDeltaSync\Audit\AuditLogger;
use KeePassDeltaSync\Audit\EventType;
use KeePassDeltaSync\Auth\AuthContext;
use KeePassDeltaSync\Config;
use KeePassDeltaSync\Crypto\TokenHasher;
use KeePassDeltaSync\Db\Connection;
use KeePassDeltaSync\Http\HttpException;
use KeePassDeltaSync\Http\JsonResponse;
use KeePassDeltaSync\Http\Request;
use KeePassDeltaSync\Http\Response;
use PDO;

/**
 * POST /api/v1/devices/enroll — bytter en enrollment-token til en permanent
 * device-token.
 *
 * Engangsbrug: en succesfuld enrollment markerer enrollment-tokenet som
 * `used = true`. Hvis to enheder forsøger samme token samtidigt vinder
 * den ene, og den anden får 401 — UPDATE ... WHERE used = false giver os
 * race-safe atomicitet uden eksplicit låsning.
 */
final class EnrollmentController
{
    public function __construct(
        private readonly PDO    $pdo,
        private readonly Config $config,
    ) {}

    /** @param array<string,string> $params */
    public function enroll(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $deviceName = $this->parseDeviceName($req);
        $publicKey  = $this->parsePublicKey($req);

        if ($auth->userId === null || $auth->enrollmentTokenHash === null) {
            // Defensiv: TokenAuthenticator skal have sat begge for enrollment-tokens.
            throw new HttpException(500, 'enrollment context incomplete');
        }

        $userId              = $auth->userId;
        $enrollmentTokenHash = $auth->enrollmentTokenHash;

        $result = Connection::transaction(
            $this->pdo,
            function (PDO $pdo) use ($userId, $enrollmentTokenHash, $deviceName, $publicKey): array {
                $upd = $pdo->prepare(
                    'UPDATE enrollment_tokens
                        SET used = true
                      WHERE token_hash = :h
                        AND used       = false
                        AND expires_at > now()'
                );
                $upd->execute(['h' => $enrollmentTokenHash]);
                if ($upd->rowCount() === 0) {
                    // Mellem auth-validering og UPDATE blev tokenet brugt eller udløb.
                    throw new HttpException(401, 'enrollment token already used or expired', 'invalid_token');
                }

                $deviceToken = TokenHasher::generate();
                $deviceHash  = TokenHasher::hash($deviceToken);

                // BYTEA passes via base64-text + decode() i SQL — samme
                // pattern som entry_versions.blob, undgår PDO-binær-mareridt.
                // decode(NULL, 'base64') returnerer NULL, så vi behøver ikke
                // CASE-conditional for legacy-enheder uden public_key.
                $ins = $pdo->prepare(
                    'INSERT INTO devices (user_id, name, token_hash, public_key)
                     VALUES (:uid, :name, :h, decode(:pk_b64, \'base64\'))
                     RETURNING id, enrolled_at'
                );
                $ins->execute([
                    'uid'    => $userId,
                    'name'   => $deviceName,
                    'h'      => $deviceHash,
                    'pk_b64' => $publicKey === null ? null : base64_encode($publicKey),
                ]);
                $row = $ins->fetch();

                return [
                    'device_id'    => $row['id'],
                    'enrolled_at'  => $row['enrolled_at'],
                    'device_token' => $deviceToken,
                ];
            },
        );

        $log->info(EventType::EnrollmentSuccess, [
            'device_id' => $result['device_id'],
            'details'   => ['device_name' => $deviceName],
        ]);

        return new JsonResponse(201, [
            'device' => [
                'id'          => $result['device_id'],
                'name'        => $deviceName,
                'enrolled_at' => $result['enrolled_at'],
            ],
            'token' => $result['device_token'],
        ]);
    }

    private function parseDeviceName(Request $req): ?string
    {
        $body = $req->jsonBody();
        if (!array_key_exists('device_name', $body)) {
            return null;
        }
        $name = $body['device_name'];
        if ($name === null) {
            return null;
        }
        if (!is_string($name)) {
            throw new HttpException(400, 'device_name must be a string', 'invalid_body');
        }
        $name = trim($name);
        if ($name === '') {
            return null;
        }
        if (strlen($name) > 200) {
            throw new HttpException(400, 'device_name too long (max 200 chars)', 'invalid_body');
        }
        return $name;
    }

    /**
     * parsePublicKey læser den valgfri public_key fra request body. Forventet
     * format: base64-encoded 32 bytes (X25519). Tom/manglende felt returnerer
     * null — klienten kan så uploade key senere via PATCH /me. Ugyldig
     * længde eller invalid base64 giver 400.
     */
    private function parsePublicKey(Request $req): ?string
    {
        $body = $req->jsonBody();
        if (!array_key_exists('public_key', $body)) {
            return null;
        }
        $val = $body['public_key'];
        if ($val === null || $val === '') {
            return null;
        }
        if (!is_string($val)) {
            throw new HttpException(400, 'public_key must be a base64 string', 'invalid_body');
        }
        $decoded = base64_decode($val, true);
        if ($decoded === false) {
            throw new HttpException(400, 'public_key is not valid base64', 'invalid_body');
        }
        if (strlen($decoded) !== 32) {
            throw new HttpException(400, 'public_key must decode to 32 bytes (X25519)', 'invalid_body');
        }
        return $decoded;
    }
}
