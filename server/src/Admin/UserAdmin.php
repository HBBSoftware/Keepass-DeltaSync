<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Admin;

use KeePassDeltaSync\Crypto\TokenHasher;
use KeePassDeltaSync\Db\Connection;
use PDO;

/**
 * Forretningslogik til bruger-administration. Bruges af både:
 *
 *   - `bin/admin user:*`-kommandoer (via `AdminCli`)
 *   - `POST/GET/PATCH/DELETE /admin/users` + `POST .../enrollment` (via
 *     `Controllers\Admin\UserController`)
 *
 * Validering af input (username-format, display_name-længde, UUID-format)
 * sker bevidst i kald-laget — service'n stoler på input og kaster
 * `PDOException` på DB-level violations (fx unique_violation = 23505 hvis
 * username er taget). Kald-laget mapper det til 400/409/exit-code.
 */
final class UserAdmin
{
    public function __construct(
        private readonly PDO $pdo,
        private readonly int $enrollmentTokenTtlHours,
    ) {}

    /**
     * INSERT user + initial enrollment-token i én transaktion.
     *
     * @return array{
     *     user: array{id:string, username:string, display_name:?string, created_at:string},
     *     enrollment_token: string,
     *     expires_at: string,
     * }
     * @throws \PDOException SQLSTATE 23505 hvis username er taget
     */
    public function createUserWithEnrollment(string $username, ?string $displayName): array
    {
        return Connection::transaction(
            $this->pdo,
            function (PDO $pdo) use ($username, $displayName): array {
                $ins = $pdo->prepare(
                    'INSERT INTO users (username, display_name)
                     VALUES (:u, :d)
                     RETURNING id, username, display_name, created_at'
                );
                $ins->execute(['u' => $username, 'd' => $displayName]);
                $user = $ins->fetch();

                [$token, $expiresAt] = $this->insertEnrollmentToken($pdo, (string) $user['id']);

                return [
                    'user' => [
                        'id'           => $user['id'],
                        'username'     => $user['username'],
                        'display_name' => $user['display_name'],
                        'created_at'   => $user['created_at'],
                    ],
                    'enrollment_token' => $token,
                    'expires_at'       => $expiresAt,
                ];
            },
        );
    }

    /**
     * Ny enrollment-token til eksisterende bruger.
     *
     * @return array{enrollment_token:string, expires_at:string}|null
     *         null hvis bruger ikke findes eller er deaktiveret.
     */
    public function createEnrollmentToken(string $userId): ?array
    {
        $stmt = $this->pdo->prepare(
            'SELECT id FROM users WHERE id = :id AND disabled = false'
        );
        $stmt->execute(['id' => $userId]);
        if ($stmt->fetchColumn() === false) {
            return null;
        }

        [$token, $expiresAt] = $this->insertEnrollmentToken($this->pdo, $userId);
        return ['enrollment_token' => $token, 'expires_at' => $expiresAt];
    }

    /**
     * @return list<array{
     *     id:string, username:string, display_name:?string, created_at:string,
     *     disabled:bool, device_count:int, database_count:int
     * }>
     */
    public function listUsers(): array
    {
        $stmt = $this->pdo->query(
            'SELECT u.id, u.username, u.display_name, u.created_at, u.disabled,
                    (SELECT count(*) FROM devices   d  WHERE d.user_id  = u.id) AS device_count,
                    (SELECT count(*) FROM databases db WHERE db.user_id = u.id) AS database_count
               FROM users u
              ORDER BY u.username'
        );
        return array_map(fn(array $r): array => [
            'id'             => $r['id'],
            'username'       => $r['username'],
            'display_name'   => $r['display_name'],
            'created_at'     => $r['created_at'],
            'disabled'       => (bool) $r['disabled'],
            'device_count'   => (int) $r['device_count'],
            'database_count' => (int) $r['database_count'],
        ], $stmt->fetchAll());
    }

    /** @return bool true hvis bruger findes og blev opdateret. */
    public function setDisabled(string $userId, bool $disabled): bool
    {
        $stmt = $this->pdo->prepare('UPDATE users SET disabled = :d WHERE id = :id');
        $stmt->bindValue(':d',  $disabled, PDO::PARAM_BOOL);
        $stmt->bindValue(':id', $userId);
        $stmt->execute();
        return $stmt->rowCount() > 0;
    }

    /** @return bool true hvis bruger fandtes og blev slettet (CASCADE rydder resten). */
    public function deleteUser(string $userId): bool
    {
        $stmt = $this->pdo->prepare('DELETE FROM users WHERE id = :id');
        $stmt->execute(['id' => $userId]);
        return $stmt->rowCount() > 0;
    }

    /** Slå brugernavn op til ID. Returnerer null hvis ikke findes eller er deaktiveret. */
    public function findUserByUsername(string $username): ?string
    {
        $stmt = $this->pdo->prepare(
            'SELECT id FROM users WHERE username = :u AND disabled = false'
        );
        $stmt->execute(['u' => $username]);
        $id = $stmt->fetchColumn();
        return $id === false ? null : (string) $id;
    }

    /** @return array{0:string, 1:string}  [token-klartekst, expires_at] */
    private function insertEnrollmentToken(PDO $pdo, string $userId): array
    {
        $token = TokenHasher::generate();
        $hash  = TokenHasher::hash($token);
        $exp   = (new \DateTimeImmutable('+' . $this->enrollmentTokenTtlHours . ' hours'))
            ->format('Y-m-d H:i:sP');

        $stmt = $pdo->prepare(
            'INSERT INTO enrollment_tokens (token_hash, user_id, expires_at)
             VALUES (:h, :uid, :exp)'
        );
        $stmt->execute(['h' => $hash, 'uid' => $userId, 'exp' => $exp]);

        return [$token, $exp];
    }
}
