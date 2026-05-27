<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Controllers;

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
 * Entry-sync på 6 endpoints. Selve den krypterede payload er opaque for
 * serveren — vi modtager og returnerer den som base64, og lagrer den som
 * BYTEA via PostgreSQL's encode/decode-funktioner (ingen binær-streams i
 * PDO-laget).
 *
 * Cross-user isolation: alle metoder begynder med `resolveDatabase()`, som
 * verificerer ejerskab og returnerer 404 hvis databasen ikke findes eller
 * tilhører en anden bruger.
 *
 * Atomicitet: `insertNewVersion()` bumper `database_seq.next_seq` og
 * indsætter en ny `entry_versions`-række i samme transaktion. Triggeren
 * fra migration 005 rotererer derefter version_num 3→2, 2→1, dropper en
 * evt. tidligere 1.
 *
 * Race-håndtering: hvis databasen slettes under en igangværende
 * skrivning får vi FK-violation (SQLSTATE 23503) som mappes til 404.
 */
final class EntryController
{
    /** Max blob-størrelse pr. entry. 1 MB er rigeligt selv med attachments. */
    private const MAX_BLOB_BYTES = 1024 * 1024;

    public function __construct(
        private readonly PDO    $pdo,
        private readonly Config $config,
    ) {}

    // ============================================================
    // Endpoints
    // ============================================================

    /** GET /databases/{id}/changes?since={seq} */
    public function changes(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $databaseId = $this->resolveDatabase($params['id'] ?? '', $auth);
        $since      = $this->parseSince($req);

        $stmt = $this->pdo->prepare(
            'SELECT ev.entry_uuid,
                    encode(ev.blob, \'base64\') AS blob,
                    ev.modified_at,
                    ev.deleted,
                    ev.server_seq,
                    (SELECT count(*) FROM entry_versions ev2
                      WHERE ev2.database_id = ev.database_id
                        AND ev2.entry_uuid  = ev.entry_uuid) AS available_versions
               FROM entry_versions ev
              WHERE ev.database_id = :db
                AND ev.version_num = 3
                AND ev.server_seq  > :since
              ORDER BY ev.server_seq ASC'
        );
        $stmt->execute(['db' => $databaseId, 'since' => $since]);

        $entries = array_map(fn(array $r): array => [
            'uuid'               => $r['entry_uuid'],
            'blob'               => $r['blob'],
            'modified_at'        => self::isoUtc($r['modified_at']),
            'deleted'            => (bool) $r['deleted'],
            'seq'                => (int)  $r['server_seq'],
            'available_versions' => (int)  $r['available_versions'],
        ], $stmt->fetchAll());

        $cur = $this->pdo->prepare(
            'SELECT next_seq - 1 AS current_seq FROM database_seq WHERE database_id = :db'
        );
        $cur->execute(['db' => $databaseId]);
        $currentSeq = (int) ($cur->fetchColumn() ?: 0);

        $log->debug(EventType::EntriesChangesFetched, [
            'database_id' => $databaseId,
            'details'     => ['since' => $since, 'count' => count($entries), 'current_seq' => $currentSeq],
        ]);

        return new JsonResponse(200, [
            'current_seq' => $currentSeq,
            'entries'     => $entries,
        ]);
    }

    /** PUT /databases/{id}/entries/{uuid} */
    public function put(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $databaseId = $this->resolveDatabase($params['id'] ?? '', $auth);
        $entryUuid  = $this->validateEntryUuid($params['uuid'] ?? '');

        $body       = $req->jsonBody();
        $blobB64    = $this->parseBlob($body, required: true);
        $modifiedAt = $this->parseModifiedAt($body);

        $result = $this->insertNewVersion($databaseId, $entryUuid, $blobB64, $modifiedAt, deleted: false);

        $log->debug(EventType::EntryPut, [
            'database_id' => $databaseId,
            'entry_uuid'  => $entryUuid,
            'details'     => ['seq' => $result['seq']],
        ]);

        return new JsonResponse(200, [
            'entry' => [
                'uuid'        => $entryUuid,
                'modified_at' => self::isoUtc($modifiedAt),
                'deleted'     => false,
                'seq'         => $result['seq'],
                'created_at'  => self::isoUtc($result['created_at']),
            ],
        ]);
    }

    /** DELETE /databases/{id}/entries/{uuid} — tombstone (også en version) */
    public function destroy(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $databaseId = $this->resolveDatabase($params['id'] ?? '', $auth);
        $entryUuid  = $this->validateEntryUuid($params['uuid'] ?? '');

        $body       = $req->jsonBody();
        $blobB64    = $this->parseBlob($body, required: false) ?? '';
        $modifiedAt = $this->parseModifiedAt($body);

        $result = $this->insertNewVersion($databaseId, $entryUuid, $blobB64, $modifiedAt, deleted: true);

        $log->debug(EventType::EntryDeleted, [
            'database_id' => $databaseId,
            'entry_uuid'  => $entryUuid,
            'details'     => ['seq' => $result['seq']],
        ]);

        return new JsonResponse(200, [
            'entry' => [
                'uuid'        => $entryUuid,
                'modified_at' => self::isoUtc($modifiedAt),
                'deleted'     => true,
                'seq'         => $result['seq'],
                'created_at'  => self::isoUtc($result['created_at']),
            ],
        ]);
    }

    /** GET /databases/{id}/entries/{uuid}/versions */
    public function versions(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $databaseId = $this->resolveDatabase($params['id'] ?? '', $auth);
        $entryUuid  = $this->validateEntryUuid($params['uuid'] ?? '');

        $stmt = $this->pdo->prepare(
            'SELECT version_num,
                    encode(blob, \'base64\') AS blob,
                    modified_at,
                    created_at,
                    deleted
               FROM entry_versions
              WHERE database_id = :db AND entry_uuid = :uuid
              ORDER BY version_num DESC'
        );
        $stmt->execute(['db' => $databaseId, 'uuid' => $entryUuid]);
        $rows = $stmt->fetchAll();

        if (!$rows) {
            throw new HttpException(404, 'entry not found', 'not_found');
        }

        $versions = array_map(fn(array $r): array => [
            'version_num' => (int) $r['version_num'],
            'modified_at' => self::isoUtc($r['modified_at']),
            'created_at'  => self::isoUtc($r['created_at']),
            'deleted'     => (bool) $r['deleted'],
            'blob'        => $r['blob'],
        ], $rows);

        $log->debug(EventType::EntryVersionsListed, [
            'database_id' => $databaseId,
            'entry_uuid'  => $entryUuid,
            'details'     => ['count' => count($versions)],
        ]);

        return new JsonResponse(200, [
            'entry_uuid' => $entryUuid,
            'versions'   => $versions,
        ]);
    }

    /** GET /databases/{id}/entries/{uuid}/versions/{num} */
    public function version(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $databaseId = $this->resolveDatabase($params['id'] ?? '', $auth);
        $entryUuid  = $this->validateEntryUuid($params['uuid'] ?? '');
        $versionNum = $this->validateVersionNum($params['num'] ?? '');

        $stmt = $this->pdo->prepare(
            'SELECT version_num,
                    encode(blob, \'base64\') AS blob,
                    modified_at,
                    created_at,
                    deleted
               FROM entry_versions
              WHERE database_id = :db
                AND entry_uuid  = :uuid
                AND version_num = :num'
        );
        $stmt->execute(['db' => $databaseId, 'uuid' => $entryUuid, 'num' => $versionNum]);
        $row = $stmt->fetch();

        if (!$row) {
            throw new HttpException(404, 'version not found', 'not_found');
        }

        $log->debug(EventType::EntryVersionFetched, [
            'database_id' => $databaseId,
            'entry_uuid'  => $entryUuid,
            'details'     => ['version_num' => $versionNum],
        ]);

        return new JsonResponse(200, [
            'version_num' => (int) $row['version_num'],
            'modified_at' => self::isoUtc($row['modified_at']),
            'created_at'  => self::isoUtc($row['created_at']),
            'deleted'     => (bool) $row['deleted'],
            'blob'        => $row['blob'],
        ]);
    }

    /**
     * POST /databases/{id}/entries/{uuid}/restore/{num}
     *
     * Kopierer en gammel version's blob ind som ny nyeste version. modified_at
     * sættes til server's now() (ikke den restored versions oprindelige
     * timestamp), så ændringen vinder under KeePassXC's merge på klienterne.
     * Dette afviger fra spec'ens "kopiér" formulering, men er nødvendigt for
     * at restore semantisk *gør noget* når der findes nyere edits på andre
     * enheder — ellers ville restore tabe merge'en til den nyere edit.
     */
    public function restore(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $databaseId = $this->resolveDatabase($params['id'] ?? '', $auth);
        $entryUuid  = $this->validateEntryUuid($params['uuid'] ?? '');
        $versionNum = $this->validateVersionNum($params['num'] ?? '');

        $now = (new \DateTimeImmutable())->format('Y-m-d\TH:i:sP');

        $result = $this->withFkSafety(fn() => Connection::transaction(
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

                $inserted = $this->insertNewVersionInTransaction(
                    $pdo,
                    $databaseId,
                    $entryUuid,
                    $row['blob'],
                    $now,
                    (bool) $row['deleted'],
                );
                $inserted['deleted'] = (bool) $row['deleted'];
                return $inserted;
            },
        ));

        $log->debug(EventType::EntryRestored, [
            'database_id' => $databaseId,
            'entry_uuid'  => $entryUuid,
            'details'     => [
                'restored_from' => $versionNum,
                'new_seq'       => $result['seq'],
                'deleted'       => $result['deleted'],
            ],
        ]);

        return new JsonResponse(200, [
            'entry' => [
                'uuid'          => $entryUuid,
                'modified_at'   => self::isoUtc($now),
                'deleted'       => $result['deleted'],
                'seq'           => $result['seq'],
                'created_at'    => self::isoUtc($result['created_at']),
                'restored_from' => $versionNum,
            ],
        ]);
    }

    // ============================================================
    // Helpers
    // ============================================================

    private function resolveDatabase(string $id, AuthContext $auth): string
    {
        if (!self::isUuid($id)) {
            throw new HttpException(404, 'database not found', 'not_found');
        }
        $stmt = $this->pdo->prepare(
            'SELECT id FROM databases WHERE id = :id AND user_id = :uid'
        );
        $stmt->execute(['id' => $id, 'uid' => $auth->userId]);
        $found = $stmt->fetchColumn();
        if ($found === false) {
            throw new HttpException(404, 'database not found', 'not_found');
        }
        return (string) $found;
    }

    private function validateEntryUuid(string $uuid): string
    {
        if (!self::isUuid($uuid)) {
            throw new HttpException(404, 'entry not found', 'not_found');
        }
        return $uuid;
    }

    private function validateVersionNum(string $num): int
    {
        if (!preg_match('/^[123]$/', $num)) {
            throw new HttpException(404, 'version not found', 'not_found');
        }
        return (int) $num;
    }

    private function parseSince(Request $req): int
    {
        $raw = (string) ($req->query['since'] ?? '0');
        if (!ctype_digit($raw)) {
            throw new HttpException(400, 'since must be a non-negative integer', 'invalid_query');
        }
        return (int) $raw;
    }

    /** Returnerer normaliseret base64-streng (eller null hvis ikke required + ikke i body). */
    private function parseBlob(array $body, bool $required): ?string
    {
        $blob = $body['blob'] ?? null;
        if ($blob === null) {
            if ($required) {
                throw new HttpException(400, 'blob is required', 'invalid_body');
            }
            return null;
        }
        if (!is_string($blob)) {
            throw new HttpException(400, 'blob must be a base64 string', 'invalid_body');
        }
        $decoded = base64_decode($blob, true);
        if ($decoded === false) {
            throw new HttpException(400, 'blob is not valid base64', 'invalid_body');
        }
        if (strlen($decoded) > self::MAX_BLOB_BYTES) {
            throw new HttpException(400, 'blob too large (max ' . self::MAX_BLOB_BYTES . ' bytes)', 'invalid_body');
        }
        // Normalisér: re-encode for at sikre konsistent padding/alfabet før vi
        // sender videre til PG's decode(:blob, 'base64').
        return base64_encode($decoded);
    }

    /** Validerer + normaliserer modified_at til ISO 8601 med offset. */
    private function parseModifiedAt(array $body): string
    {
        $mod = $body['modified_at'] ?? null;
        if (!is_string($mod)) {
            throw new HttpException(400, 'modified_at is required (ISO 8601 with timezone)', 'invalid_body');
        }
        // Prøv ATOM (RFC 3339 med offset), så ISO 8601 med Z-suffix (uden + uden µs).
        $dt = \DateTimeImmutable::createFromFormat(\DateTimeImmutable::ATOM, $mod)
           ?: \DateTimeImmutable::createFromFormat('Y-m-d\TH:i:s\Z', $mod)
           ?: \DateTimeImmutable::createFromFormat('Y-m-d\TH:i:s.u\Z', $mod);
        if ($dt === false) {
            throw new HttpException(400, 'modified_at must be ISO 8601 with timezone', 'invalid_body');
        }
        return $dt->format('Y-m-d\TH:i:sP');
    }

    /**
     * @return array{seq:int, created_at:string}
     */
    private function insertNewVersion(string $databaseId, string $entryUuid, string $blobB64, string $modifiedAt, bool $deleted): array
    {
        return $this->withFkSafety(fn() => Connection::transaction(
            $this->pdo,
            fn(PDO $pdo) => $this->insertNewVersionInTransaction(
                $pdo, $databaseId, $entryUuid, $blobB64, $modifiedAt, $deleted,
            ),
        ));
    }

    /**
     * Antager at vi allerede er i en transaktion.
     *
     * @return array{seq:int, created_at:string}
     */
    private function insertNewVersionInTransaction(PDO $pdo, string $databaseId, string $entryUuid, string $blobB64, string $modifiedAt, bool $deleted): array
    {
        // entries-rækken er FK-target for entry_versions. Idempotent UPSERT.
        $upsert = $pdo->prepare(
            'INSERT INTO entries (database_id, entry_uuid)
             VALUES (:db, :uuid)
             ON CONFLICT DO NOTHING'
        );
        $upsert->execute(['db' => $databaseId, 'uuid' => $entryUuid]);

        // Bump server_seq og returnér den værdi vi skal bruge til denne version.
        $seqStmt = $pdo->prepare(
            'UPDATE database_seq
                SET next_seq = next_seq + 1
              WHERE database_id = :db
              RETURNING next_seq - 1 AS seq'
        );
        $seqStmt->execute(['db' => $databaseId]);
        $seq = (int) $seqStmt->fetchColumn();

        // INSERT — version-rotation-triggeren fra migration 005 sørger for 3→2→1.
        $ins = $pdo->prepare(
            'INSERT INTO entry_versions
                 (database_id, entry_uuid, blob, modified_at, deleted, server_seq, version_num)
             VALUES (:db, :uuid, decode(:blob, \'base64\'), :mod, :del, :seq, 3)
             RETURNING created_at'
        );
        $ins->bindValue(':db',   $databaseId);
        $ins->bindValue(':uuid', $entryUuid);
        $ins->bindValue(':blob', $blobB64);
        $ins->bindValue(':mod',  $modifiedAt);
        $ins->bindValue(':del',  $deleted, PDO::PARAM_BOOL);
        $ins->bindValue(':seq',  $seq, PDO::PARAM_INT);
        $ins->execute();
        $createdAt = (string) $ins->fetchColumn();

        return ['seq' => $seq, 'created_at' => $createdAt];
    }

    /**
     * Wrapper der mapper FK-violations (typisk: databasen blev slettet midt i
     * vores skrivning) til 404 i stedet for 500.
     *
     * @template T
     * @param  callable(): T $fn
     * @return T
     */
    private function withFkSafety(callable $fn): mixed
    {
        try {
            return $fn();
        } catch (\PDOException $e) {
            if ($e->getCode() === '23503') {  // foreign_key_violation
                throw new HttpException(404, 'database not found', 'not_found');
            }
            throw $e;
        }
    }

    /** Konverter PG-timestamp til ISO 8601 UTC (Z-suffix). */
    private static function isoUtc(?string $pgTimestamp): ?string
    {
        if ($pgTimestamp === null) {
            return null;
        }
        return (new \DateTimeImmutable($pgTimestamp))
            ->setTimezone(new \DateTimeZone('UTC'))
            ->format('Y-m-d\TH:i:s\Z');
    }

    private static function isUuid(string $s): bool
    {
        return (bool) preg_match(
            '/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/',
            $s,
        );
    }
}
