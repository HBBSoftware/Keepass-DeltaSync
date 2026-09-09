<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Cli;

use KeePassDeltaSync\Admin\DatabaseAdmin;
use KeePassDeltaSync\Admin\UserAdmin;
use KeePassDeltaSync\Auth\AdminAccount;
use KeePassDeltaSync\Auth\AdminSession;
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
    /** Samme minimum som container-entrypointet håndhæver. */
    private const MIN_PASSWORD_LENGTH = 12;

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
            'admin:set-password'         => $this->setAdminPassword($rest),
            'admin:ensure'               => $this->ensureAdminAccount(),
            'user:create'                => $this->createUser($rest),
            'user:enrollment'            => $this->createEnrollmentToken($rest),
            'user:list'                  => $this->listUsers(),
            'user:disable'               => $this->setUserDisabled($rest, true),
            'user:enable'                => $this->setUserDisabled($rest, false),
            'user:delete'                => $this->deleteUser($rest),
            'database:create'            => $this->createDatabase($rest),
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

        Admin-login (browser-panelet):
          admin:set-password <username>
              Sæt brugernavn og adgangskode til admin-panelet. Adgangskoden
              læses fra ADMIN_PASSWORD hvis den er sat, ellers spørges der
              interaktivt. Alle åbne sessioner logges ud.

          admin:ensure
              Opret/opdatér kontoen ud fra ADMIN_USERNAME + ADMIN_PASSWORD.
              Idempotent — kaldes af container-entrypointet ved hver opstart.

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

        Database-administration:
          database:create <username> <navn>
              Opret en database med brugeren som ejer. POST /databases kan
              kun nås med et device-token, så uden den her kan en admin
              oprette brugere men ikke den database de skal synkronisere.
              Der er ingen kryptografi i det: masternøglen udledes lokalt
              hos ejeren og når aldrig serveren.

        HELP);
        return 0;
    }

    /**
     * admin:set-password <username> — sæt legitimation til admin-panelet.
     *
     * Adgangskoden tages fra ADMIN_PASSWORD hvis den findes (så kommandoen
     * kan scriptes), ellers spørges der interaktivt med slukket echo.
     */
    private function setAdminPassword(array $args): int
    {
        $username = $args[0] ?? null;
        if (!is_string($username) || trim($username) === '') {
            fwrite(STDERR, "Anvendelse: admin admin:set-password <username>\n");
            return 2;
        }
        $username = trim($username);

        $password = getenv('ADMIN_PASSWORD');
        if (!is_string($password) || $password === '') {
            $password = $this->promptHidden('Adgangskode: ');
            $confirm  = $this->promptHidden('Gentag: ');
            if ($password !== $confirm) {
                fwrite(STDERR, "Adgangskoderne er ikke ens.\n");
                return 1;
            }
        }

        if (strlen($password) < self::MIN_PASSWORD_LENGTH) {
            fwrite(STDERR, sprintf(
                "Adgangskoden er %d tegn; mindst %d kræves.\n",
                strlen($password),
                self::MIN_PASSWORD_LENGTH,
            ));
            return 1;
        }

        try {
            $pdo = Connection::fromConfig($this->config);
            AdminAccount::set($pdo, $username, $password, $this->config);
            // En ny adgangskode skal slå eksisterende browser-sessioner ihjel,
            // ellers overlever en stjålet cookie netop den handling der er
            // ment som svaret på tyveriet.
            AdminSession::destroyAll($pdo);
        } catch (\PDOException $e) {
            return $this->reportDbError($e);
        }

        fwrite(STDOUT, "Admin-konto '$username' opdateret. Alle sessioner er logget ud.\n");
        return 0;
    }

    /**
     * admin:ensure — kontoen ud fra environment. Kaldes ved hver opstart, så
     * den skriver kun når noget faktisk er ændret.
     */
    private function ensureAdminAccount(): int
    {
        $username = getenv('ADMIN_USERNAME');
        $password = getenv('ADMIN_PASSWORD');

        if (!is_string($username) || $username === '' || !is_string($password) || $password === '') {
            // Ikke en fejl: kontoen er valgfri, og uden den falder panelet
            // tilbage på bearer-token som før.
            return 0;
        }

        if (strlen($password) < self::MIN_PASSWORD_LENGTH) {
            fwrite(STDERR, sprintf(
                "ADMIN_PASSWORD er %d tegn; mindst %d kræves.\n",
                strlen($password),
                self::MIN_PASSWORD_LENGTH,
            ));
            return 1;
        }

        try {
            $pdo     = Connection::fromConfig($this->config);
            $changed = AdminAccount::set($pdo, trim($username), $password, $this->config);
            if ($changed) {
                AdminSession::destroyAll($pdo);
                fwrite(STDOUT, "Admin-konto '" . trim($username) . "' sat fra environment.\n");
            }
        } catch (\PDOException $e) {
            return $this->reportDbError($e);
        }

        return 0;
    }

    /** Læs en linje fra stdin uden at vise den. Falder tilbage til synlig input. */
    private function promptHidden(string $prompt): string
    {
        fwrite(STDOUT, $prompt);

        $hasStty = false;
        if (function_exists('shell_exec')) {
            $probe   = shell_exec('command -v stty 2>/dev/null');
            $hasStty = is_string($probe) && trim($probe) !== '';
        }

        if ($hasStty) {
            $original = shell_exec('stty -g 2>/dev/null');
            shell_exec('stty -echo 2>/dev/null');
        }

        $line = fgets(STDIN);

        if ($hasStty) {
            if (is_string($original) && trim($original) !== '') {
                shell_exec('stty ' . trim($original) . ' 2>/dev/null');
            } else {
                shell_exec('stty echo 2>/dev/null');
            }
            fwrite(STDOUT, "\n");
        }

        return is_string($line) ? rtrim($line, "\r\n") : '';
    }

    /** database:create <username> <navn> */
    private function createDatabase(array $args): int
    {
        $username = $args[0] ?? null;
        $name     = $args[1] ?? null;

        if (!is_string($username) || trim($username) === ''
            || !is_string($name) || trim($name) === ''
        ) {
            fwrite(STDERR, "Anvendelse: admin database:create <username> <navn>\n");
            return 2;
        }
        $username = trim($username);
        $name     = trim($name);

        if (strlen($name) > DatabaseAdmin::MAX_NAME_LENGTH) {
            fwrite(STDERR, sprintf(
                "Navnet er %d tegn; højst %d tilladt.\n",
                strlen($name),
                DatabaseAdmin::MAX_NAME_LENGTH,
            ));
            return 1;
        }

        try {
            $pdo = Connection::fromConfig($this->config);
            $row = (new DatabaseAdmin($pdo))->create($username, $name);
        } catch (\RuntimeException $e) {
            fwrite(STDERR, $e->getMessage() . "\n");
            return 1;
        } catch (\PDOException $e) {
            return $this->reportDbError($e);
        }

        fwrite(STDOUT, sprintf(
            "Database '%s' oprettet med '%s' som ejer.\n  id: %s\n",
            $row['name'],
            $username,
            $row['id'],
        ));
        fwrite(STDOUT, "Klienten kan nu vælge den under 'Match til server-database'.\n");
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
