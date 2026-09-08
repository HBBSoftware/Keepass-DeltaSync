<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Controllers\Admin;

use KeePassDeltaSync\Admin\DeviceAdmin;
use KeePassDeltaSync\Audit\AuditLogger;
use KeePassDeltaSync\Auth\AuthContext;
use KeePassDeltaSync\Config;
use KeePassDeltaSync\Http\HttpException;
use KeePassDeltaSync\Http\JsonResponse;
use KeePassDeltaSync\Http\Request;
use KeePassDeltaSync\Http\Response;
use PDO;

/**
 * GET /api/v1/admin/devices[?user_id=UUID]
 *
 * Hvilke enheder findes, hvem ejer dem, og hvornår var de sidst i kontakt.
 *
 * Modsat /api/v1/devices, som kræver et device-token og kun viser kaldernes
 * egen brugers enheder, ser den her på tværs — det er hele pointen for en
 * administrator. Read-only.
 */
final class DeviceController
{
    public function __construct(
        private readonly PDO    $pdo,
        private readonly Config $config,
    ) {}

    /** @param array<string,string> $params */
    public function index(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $userId = $this->parseUserId($req);

        return new JsonResponse(200, [
            'devices' => (new DeviceAdmin($this->pdo))->listDevices($userId),
        ]);
    }

    /** Valideres her, ikke i service-laget — samme arbejdsdeling som UserAdmin. */
    private function parseUserId(Request $req): ?string
    {
        $raw = $req->query['user_id'] ?? null;
        if (!is_string($raw) || trim($raw) === '') {
            return null;
        }
        $raw = trim($raw);
        if (preg_match('/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i', $raw) !== 1) {
            throw new HttpException(400, 'user_id must be a UUID', 'invalid_query');
        }
        return $raw;
    }
}
