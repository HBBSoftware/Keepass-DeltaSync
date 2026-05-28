<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Controllers\Admin;

use KeePassDeltaSync\Audit\AuditLogger;
use KeePassDeltaSync\Audit\EventType;
use KeePassDeltaSync\Auth\AuthContext;
use KeePassDeltaSync\Config;
use KeePassDeltaSync\Db\Connection;
use KeePassDeltaSync\Http\HttpException;
use KeePassDeltaSync\Http\JsonResponse;
use KeePassDeltaSync\Http\Request;
use KeePassDeltaSync\Http\Response;
use PDO;

/**
 * POST /api/v1/admin/databases/{id}/entries/{uuid}/restore/{num}
 *
 * Recovery på vegne af en bruger. Admin kan rulle en entry tilbage uden
 * at læse selve blob-indholdet — blob'en kopieres uændret fra den ønskede
 * gamle version ind som ny nyeste version. Spec § Authorization-logik:
 * "Admin-tokens kan ikke tilgå brugeres database-indhold — de kan kun
 * administrere brugere og enheder". Vi lever op til det fordi blob'en
 * aldrig dekrypteres i hverken admin- eller user-flow.
 *
 * `modified_at` sættes til `now()` så restore vinder merge'en på klienterne
 * (samme afvigelse fra spec som user-facing restore — dokumenteret i
 * `EntryController::restore`).
 *
 * Logger som `admin.action` på INFO-niveau med owner_user_id i details,
 * så det er sporbart i audit-loggen hvem admin handlede på vegne af.
 */
final class EntryRestoreController
{
    public function __construct(
        private readonly PDO    $pdo,
        private readonly Config $config,
    ) {}

    /** @param array<string,string> $params */
    public function restore(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $databaseId    = $params['id']   ?? '';
        $entryUuid     = $params['uuid'] ?? '';
        $versionNumStr = $params['num']  ?? '';

        if (!self::isUuid($databaseId)) {
            throw new HttpException(404, 'database not found', 'not_found');
        }
        if (!self::isUuid($entryUuid)) {
            throw new HttpException(404, 'entry not found', 'not_found');
        }
        if (!preg_match('/^[123]$/', $versionNumStr)) {
            throw new HttpException(404, 'version not found', 'not_found');
        }
        $versionNum = (int) $versionNumStr;

        // Slå database + owner op uden user_id-scope — admin må alle.
        // Owner-id'en bruges til audit-log så det er klart hvem admin
        // handlede på vegne af. Efter v2's sharing-model er owner identificeret
        // via database_members.role = 'owner'. Hvis flere owners (= overdragelse
        // mid-restore), tag den der blev added først.
        $dbStmt = $this->pdo->prepare(
            "SELECT d.id,
                    (SELECT user_id FROM database_members dm
                      WHERE dm.database_id = d.id
                        AND dm.role        = 'owner'
                      ORDER BY dm.added_at ASC
                      LIMIT 1) AS owner_user_id
               FROM databases d
              WHERE d.id = :id"
        );
        $dbStmt->execute(['id' => $databaseId]);
        $db = $dbStmt->fetch();
        if (!$db) {
            throw new HttpException(404, 'database not found', 'not_found');
        }
        $ownerUserId = (string) ($db['owner_user_id'] ?? '');

        $now = (new \DateTimeImmutable())->format('Y-m-d\TH:i:sP');

        try {
            $result = Connection::transaction(
                $this->pdo,
                function (PDO $pdo) use ($databaseId, $entryUuid, $versionNum, $now): array {
                    $sel = $pdo->prepare(
                        'SELECT encode(blob, \'base64\') AS blob, deleted
                           FROM entry_versions
                          WHERE database_id = :db
                            AND entry_uuid  = :uuid
                            AND version_num = :num'
                    );
                    $sel->execute(['db' => $databaseId, 'uuid' => $entryUuid, 'num' => $versionNum]);
                    $row = $sel->fetch();
                    if (!$row) {
                        throw new HttpException(404, 'version not found', 'not_found');
                    }

                    // entries-rækken er FK-target. Idempotent UPSERT i tilfælde af
                    // edge-case hvor entries-rækken er væk men versioner stadig findes
                    // (burde ikke kunne ske med CASCADE, men forsvar i dybden).
                    $upsert = $pdo->prepare(
                        'INSERT INTO entries (database_id, entry_uuid)
                         VALUES (:db, :uuid)
                         ON CONFLICT DO NOTHING'
                    );
                    $upsert->execute(['db' => $databaseId, 'uuid' => $entryUuid]);

                    $seqStmt = $pdo->prepare(
                        'UPDATE database_seq
                            SET next_seq = next_seq + 1
                          WHERE database_id = :db
                          RETURNING next_seq - 1 AS seq'
                    );
                    $seqStmt->execute(['db' => $databaseId]);
                    $seq = (int) $seqStmt->fetchColumn();

                    $ins = $pdo->prepare(
                        'INSERT INTO entry_versions
                             (database_id, entry_uuid, blob, modified_at, deleted, server_seq, version_num)
                         VALUES (:db, :uuid, decode(:blob, \'base64\'), :mod, :del, :seq, 3)
                         RETURNING created_at'
                    );
                    $ins->bindValue(':db',   $databaseId);
                    $ins->bindValue(':uuid', $entryUuid);
                    $ins->bindValue(':blob', $row['blob']);
                    $ins->bindValue(':mod',  $now);
                    $ins->bindValue(':del',  (bool) $row['deleted'], PDO::PARAM_BOOL);
                    $ins->bindValue(':seq',  $seq, PDO::PARAM_INT);
                    $ins->execute();

                    return [
                        'seq'        => $seq,
                        'created_at' => (string) $ins->fetchColumn(),
                        'deleted'    => (bool) $row['deleted'],
                    ];
                },
            );
        } catch (\PDOException $e) {
            if ($e->getCode() === '23503') {  // foreign_key_violation
                // Databasen blev slettet midt i vores skrivning.
                throw new HttpException(404, 'database not found', 'not_found');
            }
            throw $e;
        }

        $log->info(EventType::AdminAction, [
            'database_id' => $databaseId,
            'entry_uuid'  => $entryUuid,
            'details' => [
                'action'        => 'entry_restore',
                'owner_user_id' => $ownerUserId,
                'restored_from' => $versionNum,
                'new_seq'       => $result['seq'],
                'deleted'       => $result['deleted'],
            ],
        ]);

        return new JsonResponse(200, [
            'entry' => [
                'uuid'          => $entryUuid,
                'database_id'   => $databaseId,
                'owner_user_id' => $ownerUserId,
                'modified_at'   => self::isoUtc($now),
                'deleted'       => $result['deleted'],
                'seq'           => $result['seq'],
                'created_at'    => self::isoUtc($result['created_at']),
                'restored_from' => $versionNum,
            ],
        ]);
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
