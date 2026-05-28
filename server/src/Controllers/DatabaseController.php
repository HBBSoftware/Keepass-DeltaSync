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
 * CRUD på synkroniserede databaser.
 *
 * Cross-user isolation: alle opslag og mutationer scopes til
 * `auth->userId`. Forsøg på at tilgå en anden brugers database returnerer
 * 404 (ikke 403) — vi differentierer ikke mellem "findes ikke" og
 * "tilhører en anden" så vi ikke lækker eksistens-information.
 */
final class DatabaseController
{
    public function __construct(
        private readonly PDO    $pdo,
        private readonly Config $config,
    ) {}

    /** @param array<string,string> $params */
    public function create(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $name = $this->parseName($req);

        $row = Connection::transaction(
            $this->pdo,
            function (PDO $pdo) use ($auth, $name): array {
                $ins = $pdo->prepare(
                    'INSERT INTO databases (name)
                     VALUES (:name)
                     RETURNING id, name, created_at'
                );
                $ins->execute(['name' => $name]);
                $row = $ins->fetch();

                // Creator bliver automatisk owner i database_members. wrapped_master_key
                // er NULL for owners — de bruger Argon2id-derivation lokalt.
                $member = $pdo->prepare(
                    "INSERT INTO database_members (database_id, user_id, wrapped_master_key, role, added_by)
                     VALUES (:db, :uid, NULL, 'owner', :uid)"
                );
                $member->execute(['db' => $row['id'], 'uid' => $auth->userId]);

                // Initiér server_seq-tælleren. database_seq.next_seq defaulter til 1.
                $seq = $pdo->prepare(
                    'INSERT INTO database_seq (database_id) VALUES (:id)'
                );
                $seq->execute(['id' => $row['id']]);

                return $row;
            },
        );

        $log->debug(EventType::DatabaseCreated, [
            'database_id' => $row['id'],
            'details'     => ['name' => $row['name']],
        ]);

        return new JsonResponse(201, [
            'database' => [
                'id'         => $row['id'],
                'name'       => $row['name'],
                'created_at' => $row['created_at'],
            ],
        ]);
    }

    /** @param array<string,string> $params */
    public function index(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        // Inkluder både owner- og member-databaser. Klient kan se rolle for
        // hver via 'role'-feltet (member kan ikke slette eller dele videre).
        // wrapped_master_key returneres som base64-streng for members; NULL
        // for owners (de bruger Argon2id-derivation lokalt). M4's accept-flow
        // bruger feltet til at bootstrappe en lokal kopi af delte databaser.
        $stmt = $this->pdo->prepare(
            "SELECT d.id, d.name, d.created_at, dm.role,
                    encode(dm.wrapped_master_key, 'base64') AS wrapped_master_key
               FROM databases d
               JOIN database_members dm ON dm.database_id = d.id
              WHERE dm.user_id = :uid
              ORDER BY d.name, d.created_at"
        );
        $stmt->execute(['uid' => $auth->userId]);

        $rows = array_map(static function (array $r): array {
            if ($r['wrapped_master_key'] !== null) {
                // encode() wrapper hvert 76. char med \n — strip dem så
                // klienten får clean base64.
                $r['wrapped_master_key'] = str_replace("\n", '', (string) $r['wrapped_master_key']);
            }
            return $r;
        }, $stmt->fetchAll());

        return new JsonResponse(200, [
            'databases' => $rows,
        ]);
    }

    /** @param array<string,string> $params */
    public function destroy(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $id = $params['id'] ?? '';
        if (!self::isUuid($id)) {
            // Malformede UUID'er får samme 404 som "tilhører en anden" —
            // ingen forskel udadtil.
            throw new HttpException(404, 'database not found', 'not_found');
        }

        // Kun owner kan slette databasen. Members får samme 404 som
        // ikke-medlemmer — ingen lækage af "du er medlem men ikke ejer".
        $stmt = $this->pdo->prepare(
            "DELETE FROM databases
              WHERE id = :id
                AND EXISTS (
                    SELECT 1 FROM database_members
                     WHERE database_id = :id
                       AND user_id     = :uid
                       AND role        = 'owner'
                )"
        );
        $stmt->execute(['id' => $id, 'uid' => $auth->userId]);

        if ($stmt->rowCount() === 0) {
            throw new HttpException(404, 'database not found', 'not_found');
        }

        $log->debug(EventType::DatabaseDeleted, ['database_id' => $id]);

        // CASCADE har taget sig af entries, entry_versions, database_seq
        // og database_members.
        return new Response(204, [], '');
    }

    private function parseName(Request $req): string
    {
        $body = $req->jsonBody();
        $name = $body['name'] ?? null;

        if (!is_string($name)) {
            throw new HttpException(400, 'name is required and must be a string', 'invalid_body');
        }
        $name = trim($name);
        if ($name === '') {
            throw new HttpException(400, 'name must not be empty', 'invalid_body');
        }
        if (strlen($name) > 200) {
            throw new HttpException(400, 'name too long (max 200 chars)', 'invalid_body');
        }
        return $name;
    }

    private static function isUuid(string $s): bool
    {
        return (bool) preg_match(
            '/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/',
            $s,
        );
    }
}
