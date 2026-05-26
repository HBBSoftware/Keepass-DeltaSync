<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Audit;

use KeePassDeltaSync\Auth\AuthContext;
use KeePassDeltaSync\Http\Request;
use PDO;

/**
 * Persisterer audit-log-rækker til `audit_log`-tabellen.
 *
 * Bruges sådan:
 *
 *     $scoped = $logger->forRequest($request, $authContext);
 *     $scoped->info(EventType::AuthSuccess, ['details' => ['route' => '...']]);
 *     $scoped->debug(EventType::EntryPut, ['database_id' => $dbId, 'entry_uuid' => $uuid]);
 *
 * `forRequest()` returnerer en ny logger med IP/user-agent/user_id/device_id
 * bundet på alle efterfølgende kald — så hvert log-kald kun behøver de
 * action-specifikke felter.
 *
 * Fejl ved selve INSERT'et tager IKKE hovedrequesten ned: de logges til
 * stderr via error_log og swallow'es. Audit-log er en biskop, ikke en
 * konge — bedre at miste en log-række end at fejle en bruger-request.
 */
final class AuditLogger
{
    /** @param array<string,mixed> $boundContext */
    public function __construct(
        private readonly PDO      $pdo,
        private readonly LogLevel $minLevel,
        private readonly array    $boundContext = [],
    ) {}

    public function forRequest(Request $req, ?AuthContext $auth = null): self
    {
        return new self($this->pdo, $this->minLevel, [
            'user_id'    => $auth?->userId,
            'device_id'  => $auth?->deviceId,
            'ip_address' => $req->clientIp(),
            'user_agent' => $req->userAgent(),
        ]);
    }

    /** @param array<string,mixed> $extra */
    public function info(EventType $type, array $extra = []): void
    {
        $this->write(LogLevel::Info, $type, $extra);
    }

    /** @param array<string,mixed> $extra */
    public function debug(EventType $type, array $extra = []): void
    {
        if ($this->minLevel === LogLevel::Info) {
            return;  // DEBUG-events undertrykkes i INFO-mode
        }
        $this->write(LogLevel::Debug, $type, $extra);
    }

    /** @param array<string,mixed> $extra */
    private function write(LogLevel $level, EventType $type, array $extra): void
    {
        try {
            $ctx = array_merge($this->boundContext, $extra);

            $stmt = $this->pdo->prepare(
                'INSERT INTO audit_log
                     (level, event_type, user_id, device_id, database_id, entry_uuid,
                      ip_address, user_agent, details, success)
                 VALUES (:level, :event, :user, :device, :db, :entry,
                         :ip, :ua, :details::jsonb, :success)'
            );
            $stmt->bindValue(':level',   $level->value);
            $stmt->bindValue(':event',   $type->value);
            $stmt->bindValue(':user',    $ctx['user_id']    ?? null);
            $stmt->bindValue(':device',  $ctx['device_id']  ?? null);
            $stmt->bindValue(':db',      $ctx['database_id'] ?? null);
            $stmt->bindValue(':entry',   $ctx['entry_uuid']  ?? null);
            $stmt->bindValue(':ip',      $ctx['ip_address']  ?? null);
            $stmt->bindValue(':ua',      $ctx['user_agent']  ?? null);
            $stmt->bindValue(':details', isset($ctx['details']) ? json_encode($ctx['details']) : null);
            $stmt->bindValue(':success', ($ctx['success'] ?? true), PDO::PARAM_BOOL);
            $stmt->execute();
        } catch (\Throwable $e) {
            error_log('[audit-log] write failed: ' . $e->getMessage());
        }
    }
}
