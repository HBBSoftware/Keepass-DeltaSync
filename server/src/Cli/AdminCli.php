<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Cli;

use KeePassDeltaSync\Admin\UserAdmin;
use KeePassDeltaSync\Config;
use KeePassDeltaSync\Crypto\TokenHasher;
use KeePassDeltaSync\Db\Connection;

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
            'user:list'                  => $this->listUsers(),
            'user:disable'               => $this->setUserDisabled($rest, true),
            'user:enable'                => $this->setUserDisabled($rest, false),
            'user:delete'                => $this->deleteUser($rest),
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

        Token-administration:
          token:create-admin
              Generér en ny admin-token. Token printes én gang til stdout
              og lagres kun som hash i DB.

        Bruger-administration:
          user:create <username> [--display-name=...]
              Opret bruger og generér enrollment-token til den første enhed.

          user:enrollment <username>
              Generér en ny enrollment-token til en eksisterende bruger
              (fx hvis brugeren skal tilføje en ekstra enhed).

          user:list
              Liste alle brugere med status, device- og database-tæller.

          user:disable <username>
          user:enable  <username>
              Deaktivér eller genaktivér en bruger. Deaktiverede brugere
              kan ikke autentificere, men deres data bevares.

          user:delete <username>
              Slet bruger permanent. CASCADE fjerner enheder, databaser
              og entries. Kan ikke fortrydes.

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
        if ($displayName !== null && strlen($displayName) > 200) {
            fwrite(STDERR, "display_name er for langt (max 200 tegn).\n");
            return 1;
        }

        try {
            $result = $this->userAdmin()->createUserWithEnrollment($username, $displayName);
        } catch (\PDOException $e) {
            if ($e->getCode() === '23505') {
                fwrite(STDERR, "Username '$username' eksisterer allerede.\n");
                return 2;
            }
            return $this->reportDbError($e);
        }

        $ttl = $this->config->enrollmentTokenTtlHours;
        fwrite(STDOUT, "Bruger '$username' oprettet (id: {$result['user']['id']}).\n");
        fwrite(STDOUT, "Enrollment-token (udløber om {$ttl} timer):\n\n");
        fwrite(STDOUT, "  {$result['enrollment_token']}\n\n");
        fwrite(STDOUT, "Brug på enheden: keepass-deltasync enroll {$result['enrollment_token']}\n");
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
            $admin  = $this->userAdmin();
            $userId = $admin->findUserByUsername($username);
            if ($userId === null) {
                fwrite(STDERR, "Bruger '$username' findes ikke (eller er deaktiveret).\n");
                return 1;
            }
            // Garantet ikke-null siden findUserByUsername lige har valideret brugeren.
            $result = $admin->createEnrollmentToken($userId);
        } catch (\PDOException $e) {
            return $this->reportDbError($e);
        }

        $ttl = $this->config->enrollmentTokenTtlHours;
        fwrite(STDOUT, "Ny enrollment-token til '$username' (udløber om {$ttl} timer):\n\n");
        fwrite(STDOUT, "  {$result['enrollment_token']}\n\n");
        return 0;
    }

    private function listUsers(): int
    {
        try {
            $users = $this->userAdmin()->listUsers();
        } catch (\PDOException $e) {
            return $this->reportDbError($e);
        }

        if (empty($users)) {
            fwrite(STDOUT, "(ingen brugere)\n");
            return 0;
        }

        // Kolonne-bredder beregnes ud fra data (med header som minimum).
        $usernameW = max(8, ...array_map(fn(array $u): int => strlen($u['username']), $users));
        $displayW  = max(7, ...array_map(
            fn(array $u): int => strlen((string) ($u['display_name'] ?? '-')),
            $users,
        ));

        $fmt = "%-{$usernameW}s  %-{$displayW}s  %-8s  %7s  %9s  %s\n";
        fwrite(STDOUT, sprintf(
            $fmt,
            'USERNAME', 'DISPLAY', 'STATUS', 'DEVICES', 'DATABASES', 'CREATED',
        ));

        foreach ($users as $u) {
            fwrite(STDOUT, sprintf(
                $fmt,
                $u['username'],
                $u['display_name'] ?? '-',
                $u['disabled'] ? 'disabled' : 'active',
                (string) $u['device_count'],
                (string) $u['database_count'],
                substr((string) $u['created_at'], 0, 10),
            ));
        }
        return 0;
    }

    private function setUserDisabled(array $args, bool $disabled): int
    {
        $verb = $disabled ? 'disable' : 'enable';
        if (count($args) < 1) {
            fwrite(STDERR, "Brug: admin user:$verb <username>\n");
            return 1;
        }
        $username = $args[0];

        try {
            $admin  = $this->userAdmin();
            $userId = $admin->findUserByUsername($username, includeDisabled: true);
            if ($userId === null) {
                fwrite(STDERR, "Bruger '$username' findes ikke.\n");
                return 1;
            }
            $admin->setDisabled($userId, $disabled);
        } catch (\PDOException $e) {
            return $this->reportDbError($e);
        }

        $past = $disabled ? 'deaktiveret' : 'genaktiveret';
        fwrite(STDOUT, "Bruger '$username' $past.\n");
        return 0;
    }

    private function deleteUser(array $args): int
    {
        if (count($args) < 1) {
            fwrite(STDERR, "Brug: admin user:delete <username>\n");
            return 1;
        }
        $username = $args[0];

        try {
            $admin  = $this->userAdmin();
            $userId = $admin->findUserByUsername($username, includeDisabled: true);
            if ($userId === null) {
                fwrite(STDERR, "Bruger '$username' findes ikke.\n");
                return 1;
            }
            $admin->deleteUser($userId);
        } catch (\PDOException $e) {
            return $this->reportDbError($e);
        }

        fwrite(STDOUT, "Bruger '$username' slettet. CASCADE har fjernet enheder, databaser og entries.\n");
        return 0;
    }

    private function userAdmin(): UserAdmin
    {
        return new UserAdmin(
            Connection::fromConfig($this->config),
            $this->config->enrollmentTokenTtlHours,
        );
    }

    private function reportDbError(\PDOException $e): int
    {
        fwrite(STDERR, 'Database-fejl: ' . $e->getMessage() . "\n");
        fwrite(STDERR, "Tjek at DATABASE_URL er sat og at schema/-migrationerne er kørt.\n");
        return 2;
    }
}
