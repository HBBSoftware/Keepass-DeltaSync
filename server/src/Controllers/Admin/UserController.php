<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Controllers\Admin;

use KeePassDeltaSync\Admin\UserAdmin;
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
 * Admin user-management endpoints. Krydser ikke entry-blobs — admin har
 * ingen kryptografisk adgang til indholdet, kun til bruger-metadata og
 * device-/database-metadata.
 *
 *   POST   /admin/users                            opret + initial token
 *   GET    /admin/users                            liste + status
 *   PATCH  /admin/users/{id}                       {"disabled": bool}
 *   DELETE /admin/users/{id}                       hård slet, CASCADE
 *   POST   /admin/users/{id}/enrollment            ny token til ekstra enhed
 */
final class UserController
{
    public function __construct(
        private readonly PDO    $pdo,
        private readonly Config $config,
    ) {}

    /** @param array<string,string> $params */
    public function create(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $body        = $req->jsonBody();
        $username    = $this->parseUsername($body);
        $displayName = $this->parseDisplayName($body);

        try {
            $result = $this->userAdmin()->createUserWithEnrollment($username, $displayName);
        } catch (\PDOException $e) {
            if ($e->getCode() === '23505') {
                throw new HttpException(409, "username '$username' already exists", 'conflict');
            }
            throw $e;
        }

        $log->info(EventType::UserCreated, [
            'details' => [
                'user_id'  => $result['user']['id'],
                'username' => $username,
            ],
        ]);

        return new JsonResponse(201, $result);
    }

    /** @param array<string,string> $params */
    public function index(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        return new JsonResponse(200, ['users' => $this->userAdmin()->listUsers()]);
    }

    /** @param array<string,string> $params */
    public function update(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $id   = $this->parseUserId($params);
        $body = $req->jsonBody();

        if (!array_key_exists('disabled', $body) || !is_bool($body['disabled'])) {
            throw new HttpException(400, "'disabled' (bool) is required", 'invalid_body');
        }
        $disabled = $body['disabled'];

        if (!$this->userAdmin()->setDisabled($id, $disabled)) {
            throw new HttpException(404, 'user not found', 'not_found');
        }

        $log->info(EventType::UserDisabled, [
            'details' => ['user_id' => $id, 'disabled' => $disabled],
        ]);

        return new JsonResponse(200, ['user_id' => $id, 'disabled' => $disabled]);
    }

    /** @param array<string,string> $params */
    public function destroy(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $id = $this->parseUserId($params);

        if (!$this->userAdmin()->deleteUser($id)) {
            throw new HttpException(404, 'user not found', 'not_found');
        }

        $log->info(EventType::UserDeleted, [
            'details' => ['user_id' => $id],
        ]);

        // CASCADE har slettet devices, databases, entries, audit_log-rækker.
        return new Response(204, [], '');
    }

    /** @param array<string,string> $params */
    public function enrollment(Request $req, array $params, AuthContext $auth, AuditLogger $log): Response
    {
        $id     = $this->parseUserId($params);
        $result = $this->userAdmin()->createEnrollmentToken($id);

        if ($result === null) {
            throw new HttpException(404, 'user not found or disabled', 'not_found');
        }

        $log->info(EventType::AdminAction, [
            'details' => ['action' => 'enrollment_token_issued', 'user_id' => $id],
        ]);

        return new JsonResponse(201, $result);
    }

    private function userAdmin(): UserAdmin
    {
        return new UserAdmin($this->pdo, $this->config->enrollmentTokenTtlHours);
    }

    /** @param array<string,mixed> $body */
    private function parseUsername(array $body): string
    {
        $u = $body['username'] ?? null;
        if (!is_string($u)) {
            throw new HttpException(400, 'username is required and must be a string', 'invalid_body');
        }
        if (!preg_match('/^[A-Za-z0-9_.\-]{1,64}$/', $u)) {
            throw new HttpException(400, 'username must be 1-64 chars from [A-Za-z0-9_.-]', 'invalid_body');
        }
        return $u;
    }

    /** @param array<string,mixed> $body */
    private function parseDisplayName(array $body): ?string
    {
        if (!array_key_exists('display_name', $body)) {
            return null;
        }
        $d = $body['display_name'];
        if ($d === null || $d === '') {
            return null;
        }
        if (!is_string($d)) {
            throw new HttpException(400, 'display_name must be a string', 'invalid_body');
        }
        if (strlen($d) > 200) {
            throw new HttpException(400, 'display_name too long (max 200 chars)', 'invalid_body');
        }
        return $d;
    }

    /** @param array<string,string> $params */
    private function parseUserId(array $params): string
    {
        $id = $params['id'] ?? '';
        if (!self::isUuid($id)) {
            throw new HttpException(404, 'user not found', 'not_found');
        }
        return $id;
    }

    private static function isUuid(string $s): bool
    {
        return (bool) preg_match(
            '/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/',
            $s,
        );
    }
}
