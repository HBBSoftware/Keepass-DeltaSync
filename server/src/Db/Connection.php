<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Db;

use KeePassDeltaSync\Config;
use PDO;

/**
 * PDO-factory + transaktions-helper.
 *
 * Connection-objekter er state-løse; PDO-instanser oprettes ved behov og
 * injiceres i controllere/repositories. Det giver simpel testbarhed —
 * tests kan bygge en in-memory eller test-DB-PDO og sende den ind direkte.
 */
final class Connection
{
    public static function fromConfig(Config $config): PDO
    {
        return new PDO(
            $config->databaseDsn,
            $config->databaseUser,
            $config->databasePassword,
            [
                PDO::ATTR_ERRMODE            => PDO::ERRMODE_EXCEPTION,
                PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
                PDO::ATTR_EMULATE_PREPARES   => false,
            ],
        );
    }

    /**
     * Kør callback'et inden i en transaktion. Commit ved succes; rollback
     * og re-throw ved exception.
     *
     * @template T
     * @param  callable(PDO): T $fn
     * @return T
     */
    public static function transaction(PDO $pdo, callable $fn): mixed
    {
        $pdo->beginTransaction();
        try {
            $result = $fn($pdo);
            $pdo->commit();
            return $result;
        } catch (\Throwable $e) {
            $pdo->rollBack();
            throw $e;
        }
    }
}
