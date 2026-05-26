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
 * Brugerens egne devices: liste + tilbagekald.
 *
 * Parrer naturligt med /log: brugeren ser en mistænkelig IP i audit-loggen,
 * identificerer den ansvarlige enhed via `last_seen`/navn, og kalder
 * DELETE /devices/{id} for at tilbagekalde token'en. Selv-revokering af det
 * current device er tilladt (returnerer 204, men næste request fra det device
 * vil fejle 401).
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
        $stmt = $this->pdo->prepare(
            'SELECT id, name, enrolled_at, last_seen
               FROM devices
              WHERE user_id = :uid
              ORDER BY enrolled_at DESC'
        );
        $stmt->execute(['uid' => $auth->userId]);

        $devices = array_map(fn(array $r): array => [
            'id'          => $r['id'],
            'name'        => $r['name'],
            'enrolled_at' => self::isoUtc($r['enrolled_at']),
            'last_seen'   => self::isoUtc($r['last_seen']),
            'is_current'  => $r['id'] === $auth->deviceId,
        ], $stmt->fetchAll());

        return new JsonResponse(200, ['devices' => $devices]);
    }

    /** @param array<string,string> $params */
    public function destroy(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $id = $params['id'] ?? '';
        if (!self::isUuid($id)) {
            throw new HttpException(404, 'device not found', 'not_found');
        }

        $stmt = $this->pdo->prepare(
            'DELETE FROM devices WHERE id = :id AND user_id = :uid'
        );
        $stmt->execute(['id' => $id, 'uid' => $auth->userId]);

        if ($stmt->rowCount() === 0) {
            throw new HttpException(404, 'device not found', 'not_found');
        }

        $log->info(EventType::DeviceRevoked, [
            'details' => [
                'revoked_device_id'    => $id,
                'revoked_by_device_id' => $auth->deviceId,
                'self_revoke'          => $id === $auth->deviceId,
            ],
        ]);

        return new Response(204, [], '');
    }

    private static function isUuid(string $s): bool
    {
        return (bool) preg_match(
            '/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/',
            $s,
        );
    }

    private static function isoUtc(?string $pgTimestamp): ?string
    {
        if ($pgTimestamp === null) {
            return null;
        }
        return (new \DateTimeImmutable($pgTimestamp))
            ->setTimezone(new \DateTimeZone('UTC'))
            ->format('Y-m-d\TH:i:s\Z');
    }
}
