<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Controllers;

use KeePassDeltaSync\Audit\AuditLogger;
use KeePassDeltaSync\Auth\AuthContext;
use KeePassDeltaSync\Config;
use KeePassDeltaSync\Http\JsonResponse;
use KeePassDeltaSync\Http\Request;
use KeePassDeltaSync\Http\Response;
use PDO;
use Throwable;

/**
 * GET /api/v1/health — public, unauthenticated liveness/readiness probe.
 *
 * Intended for container and reverse-proxy health checks (Docker, TrueNAS,
 * Caddy/Traefik, uptime monitors). Returns 200 when the app is up and the
 * database answers; 503 if the database cannot be reached. It exposes no
 * sensitive information and is deliberately not written to the audit log
 * (these probes fire frequently and would otherwise flood it).
 */
final class HealthController
{
    public function __construct(
        private readonly PDO    $pdo,
        private readonly Config $config,
    ) {}

    /** @param array<string,string> $params */
    public function check(Request $req, array $params, ?AuthContext $auth, AuditLogger $log): Response
    {
        try {
            $this->pdo->query('SELECT 1');
        } catch (Throwable) {
            return new JsonResponse(503, [
                'status' => 'unavailable',
                'db'     => 'down',
            ]);
        }

        return new JsonResponse(200, [
            'status' => 'ok',
            'db'     => 'up',
        ]);
    }
}
