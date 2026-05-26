<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Controllers\Admin;

use KeePassDeltaSync\Audit\AuditLogger;
use KeePassDeltaSync\Audit\Cleanup;
use KeePassDeltaSync\Audit\EventType;
use KeePassDeltaSync\Auth\AuthContext;
use KeePassDeltaSync\Config;
use KeePassDeltaSync\Http\HttpException;
use KeePassDeltaSync\Http\JsonResponse;
use KeePassDeltaSync\Http\Request;
use KeePassDeltaSync\Http\Response;
use PDO;

/**
 * Admin-endpoints for audit-log:
 *   GET  /api/v1/admin/log                   — komplet log med filtre
 *   POST /api/v1/admin/log/cleanup           — manuel oprydning
 */
final class LogController
{
    private const int DEFAULT_LIMIT = 50;
    private const int MAX_LIMIT     = 500;

    public function __construct(
        private readonly PDO    $pdo,
        private readonly Config $config,
    ) {}

    /**
     * GET /admin/log?since=ISO&user_id=UUID&event_type=str&limit=N
     *
     * Modsat /log (bruger-endpoint) er der ingen `user_id`-scope —
     * admin ser alle rækker, og kan filtrere på en specifik bruger.
     *
     * @param array<string,string> $params
     */
    public function index(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $since        = $this->parseSince($req);
        $userIdFilter = $this->parseUserId($req);
        $eventType    = $this->parseEventType($req);
        $limit        = $this->parseLimit($req);

        $sql  = 'SELECT occurred_at, level, event_type, user_id, device_id,
                        database_id, entry_uuid, ip_address, user_agent, details, success
                   FROM audit_log
                  WHERE 1 = 1';
        $bind = [];

        if ($since !== null) {
            $sql .= ' AND occurred_at > :since';
            $bind['since'] = $since;
        }
        if ($userIdFilter !== null) {
            $sql .= ' AND user_id = :uid';
            $bind['uid'] = $userIdFilter;
        }
        if ($eventType !== null) {
            $sql .= ' AND event_type = :evt';
            $bind['evt'] = $eventType;
        }
        $sql .= ' ORDER BY occurred_at DESC LIMIT :lim';

        $stmt = $this->pdo->prepare($sql);
        foreach ($bind as $k => $v) {
            $stmt->bindValue($k, $v);
        }
        $stmt->bindValue('lim', $limit, PDO::PARAM_INT);
        $stmt->execute();

        $entries = array_map(fn(array $r): array => [
            'occurred_at' => self::isoUtc($r['occurred_at']),
            'level'       => $r['level'],
            'event_type'  => $r['event_type'],
            'user_id'     => $r['user_id'],
            'device_id'   => $r['device_id'],
            'database_id' => $r['database_id'],
            'entry_uuid'  => $r['entry_uuid'],
            'ip_address'  => $r['ip_address'],
            'user_agent'  => $r['user_agent'],
            'details'     => $r['details'] ? json_decode($r['details'], true) : null,
            'success'     => (bool) $r['success'],
        ], $stmt->fetchAll());

        return new JsonResponse(200, [
            'log'   => $entries,
            'count' => count($entries),
        ]);
    }

    /**
     * POST /admin/log/cleanup — manuel trigger af audit-/enrollment-oprydning.
     *
     * @param array<string,string> $params
     */
    public function cleanup(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        try {
            $result = Cleanup::runForced($this->pdo, $this->config->auditRetentionDays);
        } catch (\RuntimeException) {
            throw new HttpException(409, 'cleanup already in progress', 'conflict');
        }

        $log->info(EventType::AdminAction, [
            'details' => array_merge(['action' => 'log_cleanup'], $result),
        ]);

        return new JsonResponse(200, $result);
    }

    private function parseSince(Request $req): ?string
    {
        $raw = $req->query['since'] ?? null;
        if ($raw === null || $raw === '') {
            return null;
        }
        if (!is_string($raw)) {
            throw new HttpException(400, 'since must be ISO 8601', 'invalid_query');
        }
        $dt = \DateTimeImmutable::createFromFormat(\DateTimeImmutable::ATOM, $raw)
           ?: \DateTimeImmutable::createFromFormat('Y-m-d\TH:i:s\Z', $raw);
        if ($dt === false) {
            throw new HttpException(400, 'since must be ISO 8601 with timezone', 'invalid_query');
        }
        return $dt->format('Y-m-d\TH:i:sP');
    }

    private function parseUserId(Request $req): ?string
    {
        $raw = $req->query['user_id'] ?? null;
        if ($raw === null || $raw === '') {
            return null;
        }
        if (!is_string($raw) || !self::isUuid($raw)) {
            throw new HttpException(400, 'user_id must be a UUID', 'invalid_query');
        }
        return $raw;
    }

    private function parseEventType(Request $req): ?string
    {
        $raw = $req->query['event_type'] ?? null;
        if ($raw === null || $raw === '') {
            return null;
        }
        if (!is_string($raw) || strlen($raw) > 64 || !preg_match('/^[a-z][a-z0-9._]+$/', $raw)) {
            throw new HttpException(400, 'event_type invalid', 'invalid_query');
        }
        return $raw;
    }

    private function parseLimit(Request $req): int
    {
        $raw = (string) ($req->query['limit'] ?? self::DEFAULT_LIMIT);
        if (!ctype_digit($raw)) {
            throw new HttpException(400, 'limit must be a positive integer', 'invalid_query');
        }
        $n = (int) $raw;
        if ($n < 1)               $n = self::DEFAULT_LIMIT;
        if ($n > self::MAX_LIMIT) $n = self::MAX_LIMIT;
        return $n;
    }

    private static function isUuid(string $s): bool
    {
        return (bool) preg_match(
            '/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/',
            $s,
        );
    }

    private static function isoUtc(string $pgTimestamp): string
    {
        return (new \DateTimeImmutable($pgTimestamp))
            ->setTimezone(new \DateTimeZone('UTC'))
            ->format('Y-m-d\TH:i:s\Z');
    }
}
