<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Controllers;

use KeePassDeltaSync\Audit\AuditLogger;
use KeePassDeltaSync\Audit\EventType;
use KeePassDeltaSync\Auth\AuthContext;
use KeePassDeltaSync\Config;
use KeePassDeltaSync\Http\HttpException;
use KeePassDeltaSync\Http\JsonResponse;
use KeePassDeltaSync\Http\Request;
use KeePassDeltaSync\Http\Response;
use PDO;

/**
 * GET /api/v1/me — info om den authentificerede bruger og enhed.
 * PATCH /api/v1/me — opdater enhedens public_key (legacy-upgrade flow).
 *
 * Kalder kun ind med en device-token (route'en kræver det), så $auth->userId
 * og $auth->deviceId er garanteret sat efter TokenAuthenticator har valideret
 * forespørgslen.
 */
final class MeController
{
    public function __construct(
        private readonly PDO    $pdo,
        private readonly Config $config,
    ) {}

    /** @param array<string,string> $params */
    public function show(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $stmt = $this->pdo->prepare(
            "SELECT u.id                              AS user_id,
                    u.username,
                    u.display_name,
                    u.created_at                      AS user_created_at,
                    d.id                              AS device_id,
                    d.name                            AS device_name,
                    d.enrolled_at,
                    d.last_seen,
                    encode(d.public_key, 'base64')    AS device_public_key
               FROM users u
               JOIN devices d ON d.user_id = u.id
              WHERE u.id = :uid
                AND d.id = :did"
        );
        $stmt->execute(['uid' => $auth->userId, 'did' => $auth->deviceId]);
        $row = $stmt->fetch();

        if (!$row) {
            // Skulle ikke kunne ske — token blev lige valideret. Men hvis brugeren
            // bliver slettet mellem auth-validering og dette opslag (race), så har
            // vi denne fallback.
            throw new HttpException(404, 'user not found', 'not_found');
        }

        // encode() fra postgres giver \n efter hver 76 chars. Strip dem så
        // klienten får clean base64 uden wraps.
        $pubKey = $row['device_public_key'] !== null
            ? str_replace("\n", '', $row['device_public_key'])
            : null;

        return new JsonResponse(200, [
            'user' => [
                'id'           => $row['user_id'],
                'username'     => $row['username'],
                'display_name' => $row['display_name'],
                'created_at'   => $row['user_created_at'],
            ],
            'device' => [
                'id'          => $row['device_id'],
                'name'        => $row['device_name'],
                'enrolled_at' => $row['enrolled_at'],
                'last_seen'   => $row['last_seen'],
                'public_key'  => $pubKey,
            ],
        ]);
    }

    /**
     * PATCH /api/v1/me — opdater den nuværende enheds public_key.
     *
     * Use case: legacy-enhed enrolled før v2 har NULL public_key. Næste gang
     * klienten starter, detect'er den manglende key og uploader en frisk
     * via dette endpoint. Tillader også at en enhed roterer sin key (fx
     * efter mistanke om kompromittering).
     *
     * Body: { "public_key": "<base64 string, 32 bytes decoded>" }
     *
     * @param array<string,string> $params
     */
    public function update(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $publicKey = $this->parsePublicKey($req);

        $stmt = $this->pdo->prepare(
            "UPDATE devices
                SET public_key = decode(:pk_b64, 'base64')
              WHERE id = :did
                AND user_id = :uid"
        );
        $stmt->execute([
            'pk_b64' => base64_encode($publicKey),
            'did'    => $auth->deviceId,
            'uid'    => $auth->userId,
        ]);

        if ($stmt->rowCount() === 0) {
            // Skulle ikke kunne ske med valideret token, men forsigtig fallback.
            throw new HttpException(404, 'device not found', 'not_found');
        }

        $log->info(EventType::AuthSuccess, [
            'details' => ['action' => 'public_key_updated'],
        ]);

        return new JsonResponse(200, [
            'device' => [
                'id'         => $auth->deviceId,
                'public_key' => base64_encode($publicKey),
            ],
        ]);
    }

    /**
     * parsePublicKey kræver public_key som base64-encoded 32 bytes (X25519).
     * Manglende felt eller invalid format giver 400 — modsat enroll-flow'et
     * hvor public_key er valgfri, er PATCH /me ikke meningsfuld uden.
     */
    private function parsePublicKey(Request $req): string
    {
        $body = $req->jsonBody();
        if (!array_key_exists('public_key', $body)) {
            throw new HttpException(400, 'public_key is required', 'invalid_body');
        }
        $val = $body['public_key'];
        if (!is_string($val) || $val === '') {
            throw new HttpException(400, 'public_key must be a non-empty base64 string', 'invalid_body');
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
