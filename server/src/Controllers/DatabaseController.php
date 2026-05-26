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
                    'INSERT INTO databases (user_id, name)
                     VALUES (:uid, :name)
                     RETURNING id, name, created_at'
                );
                $ins->execute(['uid' => $auth->userId, 'name' => $name]);
                $row = $ins->fetch();

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
        $stmt = $this->pdo->prepare(
            'SELECT id, name, created_at
               FROM databases
              WHERE user_id = :uid
              ORDER BY name, created_at'
        );
        $stmt->execute(['uid' => $auth->userId]);

        return new JsonResponse(200, [
            'databases' => $stmt->fetchAll(),
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

        $stmt = $this->pdo->prepare(
            'DELETE FROM databases WHERE id = :id AND user_id = :uid'
        );
        $stmt->execute(['id' => $id, 'uid' => $auth->userId]);

        if ($stmt->rowCount() === 0) {
            throw new HttpException(404, 'database not found', 'not_found');
        }

        $log->debug(EventType::DatabaseDeleted, ['database_id' => $id]);

        // CASCADE har taget sig af entries, entry_versions og database_seq.
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
