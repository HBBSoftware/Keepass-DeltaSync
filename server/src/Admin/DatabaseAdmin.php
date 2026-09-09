<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Admin;

use KeePassDeltaSync\Db\Connection;
use PDO;

/**
 * Oprettelse af databaser på en brugers vegne, til `bin/admin`.
 *
 * Endpointet POST /databases kan kun nås med et DEVICE-token, fordi ejeren
 * udledes af auth-konteksten. Det efterlader en administrator uden vej ind:
 * man kan oprette brugere, men ikke den database de skal synkronisere — og
 * en klient uden en database at vælge kan ikke komme videre. Android-appen
 * siger det ligeud: "Opret én via desktop-klienten først."
 *
 * Der er ingen kryptografi i vejen for det. En database er et navn, en
 * ejer-række og en sekvenstæller; masternøglen udledes lokalt hos ejeren med
 * Argon2id og når aldrig serveren, hvilket er derfor `wrapped_master_key`
 * står NULL for owners. Så en admin kan trygt oprette rækken — hen får
 * stadig ikke adgang til noget indhold.
 */
final class DatabaseAdmin
{
    public const MAX_NAME_LENGTH = 200;

    public function __construct(private readonly PDO $pdo) {}

    /**
     * Opret en database med den angivne bruger som owner.
     *
     * Samme tre skridt som DatabaseController::create, i én transaktion:
     * rækken, ejerskabet og server_seq-tælleren. Springer man den sidste
     * over, fejler den første PUT med en fremmednøglefejl i stedet for noget
     * læseligt.
     *
     * @return array{id:string, name:string, created_at:string}
     * @throws \RuntimeException hvis brugeren ikke findes eller er deaktiveret
     */
    public function create(string $username, string $name): array
    {
        $stmt = $this->pdo->prepare('SELECT id, disabled FROM users WHERE username = :u');
        $stmt->execute(['u' => $username]);
        $user = $stmt->fetch();

        if ($user === false) {
            throw new \RuntimeException("Bruger '$username' findes ikke.");
        }
        if ((bool) $user['disabled']) {
            throw new \RuntimeException("Bruger '$username' er deaktiveret.");
        }

        return Connection::transaction(
            $this->pdo,
            function (PDO $pdo) use ($user, $name): array {
                $ins = $pdo->prepare(
                    'INSERT INTO databases (name)
                     VALUES (:name)
                     RETURNING id, name, created_at'
                );
                $ins->execute(['name' => $name]);
                $row = $ins->fetch();

                $member = $pdo->prepare(
                    "INSERT INTO database_members (database_id, user_id, wrapped_master_key, role, added_by)
                     VALUES (:db, :uid, NULL, 'owner', :uid)"
                );
                $member->execute(['db' => $row['id'], 'uid' => $user['id']]);

                $seq = $pdo->prepare('INSERT INTO database_seq (database_id) VALUES (:id)');
                $seq->execute(['id' => $row['id']]);

                return $row;
            },
        );
    }
}
