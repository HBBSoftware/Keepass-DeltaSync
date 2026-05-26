<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Cli;

use KeePassDeltaSync\Config;
use KeePassDeltaSync\Crypto\TokenHasher;
use KeePassDeltaSync\Db\Connection;
use PDO;

/**
 * Dispatcher for `bin/admin`-kommandoer.
 *
 * Implementeret:
 *   token:create-admin                              — generér ny admin-token
 *   user:create <username> [--display-name=...]     — opret bruger + enrollment-token
 *   user:enrollment <username>                      — ny enrollment-token til eksisterende bruger
 *
 * Endnu ikke implementeret (kommer i senere milepæle):
 *   user:disable | user:delete | user:list
 *   log:cleanup
 */
final class AdminCli
{
    public function __construct(private readonly Config $config) {}

    /** Entry point fra bin/admin. */
    public static function main(string $rootDir, array $args): int
    {
        try {
            $config = Config::loadFromEnv($rootDir);
            return (new self($config))->run($args);
        } catch (\Throwable $e) {
            fwrite(STDERR, 'Fejl: ' . $e->getMessage() . "\n");
            return 1;
        }
    }

    public function run(array $args): int
    {
        $command = $args[0] ?? null;
        $rest    = array_slice($args, 1);

        return match ($command) {
            'token:create-admin'         => $this->createAdminToken(),
            'user:create'                => $this->createUser($rest),
            'user:enrollment'            => $this->createEnrollmentToken($rest),
            null, '-h', '--help', 'help' => $this->printHelp(),
            default                      => $this->unknownCommand($command),
        };
    }

    private function printHelp(): int
    {
        fwrite(STDOUT, <<<HELP
        keepass-deltasync admin CLI

        Anvendelse:
          admin <kommando> [argumenter]

        Tilgængelige kommandoer:
          token:create-admin
              Generér en ny admin-token. Token printes én gang til stdout
              og lagres kun som hash i DB.

          user:create <username> [--display-name=...]
              Opret bruger og generér enrollment-token til den første enhed.
              Tokenen printes én gang og kan ikke vises igen.

          user:enrollment <username>
              Generér en ny enrollment-token til en eksisterende bruger
              (fx hvis brugeren skal tilføje en ekstra enhed).

        Planlagt (ikke implementeret endnu):
          user:disable <username>
          user:delete  <username>
          user:list
          log:cleanup

        HELP);
        return 0;
    }

    private function unknownCommand(?string $cmd): int
    {
        fwrite(STDERR, 'Ukendt kommando: ' . ($cmd ?? '(ingen)') . "\n");
        fwrite(STDERR, "Kør 'admin help' for liste over kommandoer.\n");
        return 1;
    }

    private function createAdminToken(): int
    {
        try {
            $pdo   = Connection::fromConfig($this->config);
            $token = TokenHasher::generate();
            $hash  = TokenHasher::hash($token);

            $stmt = $pdo->prepare('INSERT INTO admin_tokens (token_hash) VALUES (:hash)');
            $stmt->execute(['hash' => $hash]);
        } catch (\PDOException $e) {
            return $this->reportDbError($e);
        }

        fwrite(STDOUT, "Admin-token oprettet. Gem den NU — den vises ikke igen.\n\n");
        fwrite(STDOUT, "  $token\n\n");
        fwrite(STDOUT, "Brug i HTTP-headeren: Authorization: Bearer $token\n");
        return 0;
    }

    private function createUser(array $args): int
    {
        if (count($args) < 1) {
            fwrite(STDERR, "Brug: admin user:create <username> [--display-name=...]\n");
            return 1;
        }

        $username    = $args[0];
        $displayName = null;
        foreach (array_slice($args, 1) as $arg) {
            if (str_starts_with($arg, '--display-name=')) {
                $value = substr($arg, strlen('--display-name='));
                $displayName = $value === '' ? null : $value;
            } else {
                fwrite(STDERR, "Ukendt argument: $arg\n");
                return 1;
            }
        }

        if (!preg_match('/^[A-Za-z0-9_.\-]{1,64}$/', $username)) {
            fwrite(STDERR, "Username skal være 1-64 tegn fra [A-Za-z0-9_.-]\n");
            return 1;
        }

        try {
            $pdo = Connection::fromConfig($this->config);
            $result = Connection::transaction(
                $pdo,
                function (PDO $pdo) use ($username, $displayName): array {
                    $ins = $pdo->prepare(
                        'INSERT INTO users (username, display_name)
                         VALUES (:u, :d) RETURNING id'
                    );
                    $ins->execute(['u' => $username, 'd' => $displayName]);
                    $userId = (string) $ins->fetchColumn();

                    $token = $this->insertEnrollmentToken($pdo, $userId);
                    return ['user_id' => $userId, 'token' => $token];
                },
            );
        } catch (\PDOException $e) {
            // SQLSTATE 23505 = unique_violation. PostgreSQL-fejlnavnet for username-uniqueness
            // ender på "_username_key", men det er ikke garanteret stabilt — tjek SQLSTATE først.
            if ($e->getCode() === '23505') {
                fwrite(STDERR, "Username '$username' eksisterer allerede.\n");
                return 2;
            }
            return $this->reportDbError($e);
        }

        $ttl = $this->config->enrollmentTokenTtlHours;
        fwrite(STDOUT, "Bruger '$username' oprettet (id: {$result['user_id']}).\n");
        fwrite(STDOUT, "Enrollment-token (udløber om {$ttl} timer):\n\n");
        fwrite(STDOUT, "  {$result['token']}\n\n");
        fwrite(STDOUT, "Brug på enheden: keepass-deltasync enroll {$result['token']}\n");
        return 0;
    }

    private function createEnrollmentToken(array $args): int
    {
        if (count($args) < 1) {
            fwrite(STDERR, "Brug: admin user:enrollment <username>\n");
            return 1;
        }
        $username = $args[0];

        try {
            $pdo  = Connection::fromConfig($this->config);
            $stmt = $pdo->prepare(
                'SELECT id FROM users WHERE username = :u AND disabled = false'
            );
            $stmt->execute(['u' => $username]);
            $userId = $stmt->fetchColumn();

            if ($userId === false) {
                fwrite(STDERR, "Bruger '$username' findes ikke (eller er deaktiveret).\n");
                return 1;
            }

            $token = $this->insertEnrollmentToken($pdo, (string) $userId);
        } catch (\PDOException $e) {
            return $this->reportDbError($e);
        }

        $ttl = $this->config->enrollmentTokenTtlHours;
        fwrite(STDOUT, "Ny enrollment-token til '$username' (udløber om {$ttl} timer):\n\n");
        fwrite(STDOUT, "  $token\n\n");
        return 0;
    }

    /** @return string den nye token (klartekst) — kun returneret én gang. */
    private function insertEnrollmentToken(PDO $pdo, string $userId): string
    {
        $token = TokenHasher::generate();
        $hash  = TokenHasher::hash($token);
        $exp   = (new \DateTimeImmutable(
            '+' . $this->config->enrollmentTokenTtlHours . ' hours'
        ))->format('Y-m-d H:i:sP');

        $stmt = $pdo->prepare(
            'INSERT INTO enrollment_tokens (token_hash, user_id, expires_at)
             VALUES (:h, :uid, :exp)'
        );
        $stmt->execute(['h' => $hash, 'uid' => $userId, 'exp' => $exp]);

        return $token;
    }

    private function reportDbError(\PDOException $e): int
    {
        fwrite(STDERR, 'Database-fejl: ' . $e->getMessage() . "\n");
        fwrite(STDERR, "Tjek at DATABASE_URL er sat og at schema/-migrationerne er kørt.\n");
        return 2;
    }
}
