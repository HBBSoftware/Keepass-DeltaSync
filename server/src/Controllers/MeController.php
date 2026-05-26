<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Controllers;

use KeePassDeltaSync\Audit\AuditLogger;
use KeePassDeltaSync\Auth\AuthContext;
use KeePassDeltaSync\Config;
use KeePassDeltaSync\Http\HttpException;
use KeePassDeltaSync\Http\JsonResponse;
use KeePassDeltaSync\Http\Request;
use KeePassDeltaSync\Http\Response;
use PDO;

/**
 * GET /api/v1/me — info om den authentificerede bruger og enhed.
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
            'SELECT u.id            AS user_id,
                    u.username,
                    u.display_name,
                    u.created_at    AS user_created_at,
                    d.id            AS device_id,
                    d.name          AS device_name,
                    d.enrolled_at,
                    d.last_seen
               FROM users u
               JOIN devices d ON d.user_id = u.id
              WHERE u.id = :uid
                AND d.id = :did'
        );
        $stmt->execute(['uid' => $auth->userId, 'did' => $auth->deviceId]);
        $row = $stmt->fetch();

        if (!$row) {
            // Skulle ikke kunne ske — token blev lige valideret. Men hvis brugeren
            // bliver slettet mellem auth-validering og dette opslag (race), så har
            // vi denne fallback.
            throw new HttpException(404, 'user not found', 'not_found');
        }

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
            ],
        ]);
    }
}
