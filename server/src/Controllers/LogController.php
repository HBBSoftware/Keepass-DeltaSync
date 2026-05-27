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
 * GET /api/v1/log — den autentificerede brugers egen audit-aktivitet.
 *
 * Bruges som privacy- og sikkerheds-værktøj: brugeren kan se hvilke IP'er
 * og user-agents der har autentificeret som dem, og opdage uautoriserede
 * enheder. Andre brugeres rækker er udenfor scope (`WHERE user_id = :uid`).
 */
final class LogController
{
    private const DEFAULT_LIMIT = 50;
    private const MAX_LIMIT     = 200;

    public function __construct(
        private readonly PDO    $pdo,
        private readonly Config $config,
    ) {}

    /** @param array<string,string> $params */
    public function index(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $since = $this->parseSince($req);
        $limit = $this->parseLimit($req);

        $sql        = 'SELECT occurred_at, level, event_type, ip_address, user_agent,
                              details, success, database_id, entry_uuid
                         FROM audit_log
                        WHERE user_id = :uid';
        $bind       = ['uid' => $auth->userId];

        if ($since !== null) {
            $sql            .= ' AND occurred_at > :since';
            $bind['since']   = $since;
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
            'ip_address'  => $r['ip_address'],
            'user_agent'  => $r['user_agent'],
            'database_id' => $r['database_id'],
            'entry_uuid'  => $r['entry_uuid'],
            'details'     => $r['details'] ? json_decode($r['details'], true) : null,
            'success'     => (bool) $r['success'],
        ], $stmt->fetchAll());

        return new JsonResponse(200, [
            'log'   => $entries,
            'count' => count($entries),
        ]);
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

    private static function isoUtc(string $pgTimestamp): string
    {
        return (new \DateTimeImmutable($pgTimestamp))
            ->setTimezone(new \DateTimeZone('UTC'))
            ->format('Y-m-d\TH:i:s\Z');
    }
}
