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
 * v2 multi-bruger sharing endpoints.
 *
 *   GET    /api/v1/users/lookup?username=X
 *   GET    /api/v1/databases/{id}/shares             (owner only)
 *   POST   /api/v1/databases/{id}/shares             (owner only)
 *   DELETE /api/v1/databases/{id}/shares/{user_id}   (owner or self)
 *
 * Server lagrer wrapped_master_key som opaque BYTEA — al kryptografi
 * sker klient-side. Server kender ikke til selve master_key.
 *
 * Privacy: enhver enrolled bruger kan lookup andre brugere via username.
 * Det er pragmatisk for vores kontrollerede deployment; en opt-in
 * "findable"-flag kan tilfojes hvis vi nogensinde får offentligt
 * deployment.
 */
final class ShareController
{
    public function __construct(
        private readonly PDO    $pdo,
        private readonly Config $config,
    ) {}

    /**
     * GET /api/v1/users/lookup?username=X
     *
     * Returnerer user-info + den "target device" (nyeste enhed med non-null
     * public_key) som Alice's klient skal wrappe master_key til. Hvis ingen
     * sådan enhed findes returneres 404 med klar fejl-besked — Bob skal
     * have kørt mindst én klient-kommando for at have auto-upgrade keypair.
     *
     * @param array<string,string> $params
     */
    public function lookupUser(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $username = trim($req->query['username'] ?? '');
        if ($username === '') {
            throw new HttpException(400, 'username query parameter is required', 'invalid_query');
        }

        $stmt = $this->pdo->prepare(
            "SELECT u.id              AS user_id,
                    u.username,
                    u.display_name,
                    d.id              AS device_id,
                    d.name            AS device_name,
                    encode(d.public_key, 'base64') AS device_public_key,
                    d.enrolled_at     AS device_enrolled_at
               FROM users u
          LEFT JOIN devices d ON d.user_id = u.id
                             AND d.public_key IS NOT NULL
              WHERE u.username = :uname
                AND u.disabled = false
              ORDER BY d.enrolled_at DESC NULLS LAST
              LIMIT 1"
        );
        $stmt->execute(['uname' => $username]);
        $row = $stmt->fetch();

        if (!$row) {
            // 404 dækker både "user findes ikke" og "user findes men er
            // disabled" — ingen forskel udadtil.
            throw new HttpException(404, 'user not found', 'not_found');
        }
        if ($row['device_id'] === null) {
            // Bruger findes, men har ingen enhed med public_key endnu.
            // Klient kan vise brugeren en handlebar fejl-besked.
            throw new HttpException(409, 'user has no devices with a public key — they must run any client command first to auto-generate one', 'no_target_device');
        }

        $pubKey = str_replace("\n", '', (string) $row['device_public_key']);

        return new JsonResponse(200, [
            'user' => [
                'id'           => $row['user_id'],
                'username'     => $row['username'],
                'display_name' => $row['display_name'],
            ],
            'target_device' => [
                'id'          => $row['device_id'],
                'name'        => $row['device_name'],
                'public_key'  => $pubKey,
                'enrolled_at' => $row['device_enrolled_at'],
            ],
        ]);
    }

    /**
     * GET /api/v1/databases/{id}/shares — list members.
     *
     * Owner-only. Members kan se sig selv via GET /databases-respons; for
     * at se hele medlems-listen skal man være owner.
     *
     * @param array<string,string> $params
     */
    public function index(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $databaseId = $this->requireOwner($params['id'] ?? '', $auth);

        $stmt = $this->pdo->prepare(
            "SELECT dm.user_id,
                    u.username,
                    u.display_name,
                    dm.role,
                    dm.added_at,
                    dm.added_by
               FROM database_members dm
               JOIN users u ON u.id = dm.user_id
              WHERE dm.database_id = :db
              ORDER BY dm.role DESC, dm.added_at ASC"
        );
        $stmt->execute(['db' => $databaseId]);

        return new JsonResponse(200, [
            'members' => $stmt->fetchAll(),
        ]);
    }

    /**
     * POST /api/v1/databases/{id}/shares
     * Body: { "user_id": "uuid", "wrapped_master_key": "base64" }
     *
     * Tilføjer en member-row. Caller skal være owner. Re-share (samme user
     * to gange) opdaterer wrapped_master_key — det er hvordan vi rotater
     * når Bob får ny enhed.
     *
     * @param array<string,string> $params
     */
    public function create(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $databaseId = $this->requireOwner($params['id'] ?? '', $auth);

        $body            = $req->jsonBody();
        $targetUserId    = $this->parseUserId($body['user_id'] ?? null);
        $wrappedKeyB64   = $this->parseWrappedKey($body['wrapped_master_key'] ?? null);

        if ($targetUserId === $auth->userId) {
            throw new HttpException(400, 'cannot share with yourself (you are already owner)', 'invalid_body');
        }

        // ON CONFLICT DO UPDATE — re-share roterer wrapped_master_key til
        // den enhed der nu skal modtage. role forbliver 'member' (vi
        // promoverer ikke ved re-share).
        $stmt = $this->pdo->prepare(
            "INSERT INTO database_members
                 (database_id, user_id, wrapped_master_key, role, added_by)
             VALUES (:db, :uid, decode(:wk, 'base64'), 'member', :by)
             ON CONFLICT (database_id, user_id) DO UPDATE
                SET wrapped_master_key = EXCLUDED.wrapped_master_key,
                    added_by           = EXCLUDED.added_by
                WHERE database_members.role = 'member'"
        );
        $stmt->execute([
            'db'  => $databaseId,
            'uid' => $targetUserId,
            'wk'  => $wrappedKeyB64,
            'by'  => $auth->userId,
        ]);

        if ($stmt->rowCount() === 0) {
            // Eneste mulighed: target er allerede owner (CONFLICT WHERE
            // role='member' matchede ikke). Returnerer 409.
            throw new HttpException(409, 'user is already owner of this database', 'conflict');
        }

        $log->info(EventType::DatabaseCreated, [
            'database_id' => $databaseId,
            'details'     => ['action' => 'shared', 'target_user_id' => $targetUserId],
        ]);

        return new JsonResponse(201, [
            'share' => [
                'database_id' => $databaseId,
                'user_id'     => $targetUserId,
                'role'        => 'member',
            ],
        ]);
    }

    /**
     * DELETE /api/v1/databases/{id}/shares/{user_id}
     *
     * Owner kan fjerne enhver member (inkl. owner-rolle? nej, owner kan
     * ikke unshare sig selv — overdragelse er v2.1-feature). Member kan
     * fjerne sig selv (= "forlade delt database").
     *
     * @param array<string,string> $params
     */
    public function destroy(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $databaseId = $params['id']      ?? '';
        $targetUid  = $params['user_id'] ?? '';
        if (!self::isUuid($databaseId) || !self::isUuid($targetUid)) {
            throw new HttpException(404, 'not found', 'not_found');
        }

        $isOwner = $this->isOwner($databaseId, (string) $auth->userId);
        $isSelf  = $targetUid === (string) $auth->userId;
        if (!$isOwner && !$isSelf) {
            // Hverken owner-handling eller self-leave — ingen ret.
            throw new HttpException(404, 'not found', 'not_found');
        }

        // Owner kan ikke fjernes via dette endpoint (heller ikke af sig
        // selv). Slet databasen via DELETE /databases/{id} i stedet, eller
        // brug en fremtidig overdragelses-endpoint.
        $stmt = $this->pdo->prepare(
            "DELETE FROM database_members
              WHERE database_id = :db
                AND user_id     = :uid
                AND role        = 'member'"
        );
        $stmt->execute(['db' => $databaseId, 'uid' => $targetUid]);

        if ($stmt->rowCount() === 0) {
            throw new HttpException(404, 'not found', 'not_found');
        }

        $log->info(EventType::DatabaseDeleted, [
            'database_id' => $databaseId,
            'details'     => [
                'action'          => 'unshared',
                'target_user_id'  => $targetUid,
                'initiated_by'    => $isSelf ? 'self' : 'owner',
            ],
        ]);

        return new Response(204, [], '');
    }

    // ============================================================
    // Helpers
    // ============================================================

    /**
     * requireOwner validerer at caller'en er owner af databasen.
     * Returnerer database-UUID som string ved succes; ellers 404.
     */
    private function requireOwner(string $databaseId, AuthContext $auth): string
    {
        if (!self::isUuid($databaseId)) {
            throw new HttpException(404, 'database not found', 'not_found');
        }
        if (!$this->isOwner($databaseId, (string) $auth->userId)) {
            throw new HttpException(404, 'database not found', 'not_found');
        }
        return $databaseId;
    }

    private function isOwner(string $databaseId, string $userId): bool
    {
        $stmt = $this->pdo->prepare(
            "SELECT 1 FROM database_members
              WHERE database_id = :db
                AND user_id     = :uid
                AND role        = 'owner'"
        );
        $stmt->execute(['db' => $databaseId, 'uid' => $userId]);
        return (bool) $stmt->fetchColumn();
    }

    private function parseUserId(mixed $val): string
    {
        if (!is_string($val) || !self::isUuid($val)) {
            throw new HttpException(400, 'user_id is required and must be a UUID', 'invalid_body');
        }
        return $val;
    }

    private function parseWrappedKey(mixed $val): string
    {
        if (!is_string($val) || $val === '') {
            throw new HttpException(400, 'wrapped_master_key is required (base64 string)', 'invalid_body');
        }
        $decoded = base64_decode($val, true);
        if ($decoded === false) {
            throw new HttpException(400, 'wrapped_master_key is not valid base64', 'invalid_body');
        }
        // Sealed-box overhead: 32-byte ephemeral pub + 16-byte mac = 48 bytes
        // for en 0-byte plaintext. master_key er 32 bytes → 80 bytes total.
        // Vi accepterer 48-512 bytes så vi har lidt slack uden at acceptere
        // urimelige paylods.
        $len = strlen($decoded);
        if ($len < 48 || $len > 512) {
            throw new HttpException(400, "wrapped_master_key has unexpected size ($len bytes); expected 48-512", 'invalid_body');
        }
        return $val;
    }

    private static function isUuid(string $s): bool
    {
        return (bool) preg_match(
            '/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/',
            $s,
        );
    }
}
