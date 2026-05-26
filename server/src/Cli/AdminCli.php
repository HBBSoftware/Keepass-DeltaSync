<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Cli;

use KeePassDeltaSync\Config;
use KeePassDeltaSync\Crypto\TokenHasher;
use KeePassDeltaSync\Db\Connection;

/**
 * Dispatcher for `bin/admin`-kommandoer.
 *
 * Foreløbig implementeret:
 *   token:create-admin   — generér og indsæt en ny admin-token.
 *
 * Endnu ikke implementeret (kommer i senere milepæle):
 *   user:create | user:disable | user:delete | user:enrollment | user:list
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

        return match ($command) {
            'token:create-admin'      => $this->createAdminToken(),
            null, '-h', '--help', 'help' => $this->printHelp(),
            default                   => $this->unknownCommand($command),
        };
    }

    private function printHelp(): int
    {
        fwrite(STDOUT, <<<HELP
        keepass-deltasync admin CLI

        Anvendelse:
          admin <kommando> [argumenter]

        Tilgængelige kommandoer:
          token:create-admin   Generér en ny admin-token. Token printes én
                               gang til stdout og lagres kun som hash i DB.

        Planlagt (ikke implementeret endnu):
          user:create <username> [--display-name=...]
          user:disable <username>
          user:delete  <username>
          user:enrollment <username>
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
            $pdo  = Connection::fromConfig($this->config);
            $token = TokenHasher::generate();
            $hash  = TokenHasher::hash($token);

            $stmt = $pdo->prepare(
                'INSERT INTO admin_tokens (token_hash) VALUES (:hash)'
            );
            $stmt->execute(['hash' => $hash]);
        } catch (\PDOException $e) {
            fwrite(STDERR, 'Database-fejl: ' . $e->getMessage() . "\n");
            fwrite(STDERR, "Tjek at DATABASE_URL er sat og at schema/-migrationerne er kørt.\n");
            return 2;
        }

        fwrite(STDOUT, "Admin-token oprettet. Gem den NU — den vises ikke igen.\n\n");
        fwrite(STDOUT, "  $token\n\n");
        fwrite(STDOUT, "Brug i HTTP-headeren: Authorization: Bearer $token\n");
        return 0;
    }
}
