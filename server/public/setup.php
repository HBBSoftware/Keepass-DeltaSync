<?php
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// One-time setup script. Bootstraps den første admin-token uden behov for
// shell-adgang. Kør én gang via browseren (eller php-cli), gem tokenen,
// og SLET filen bagefter.
//
// Guarden under tjekker om der allerede findes admin-tokens — hvis ja
// returneres 403 så scriptet ikke kan misbruges efter første kørsel. Det
// gør den idempotent-sikker, men du bør stadig slette filen: den udgør
// unødvendig angrebs-flade.

declare(strict_types=1);

require __DIR__ . '/../bootstrap.php';

use KeePassDeltaSync\Config;
use KeePassDeltaSync\Crypto\TokenHasher;
use KeePassDeltaSync\Db\Connection;

header('Content-Type: text/plain; charset=utf-8');

try {
    $config = Config::loadFromEnv(__DIR__ . '/..');
} catch (\Throwable $e) {
    http_response_code(500);
    echo "ERROR: kunne ikke læse .env: " . $e->getMessage() . "\n";
    echo "Tjek at server/.env eksisterer og indeholder DATABASE_URL, DATABASE_USER, DATABASE_PASSWORD.\n";
    exit;
}

try {
    $pdo = Connection::fromConfig($config);
} catch (\Throwable $e) {
    http_response_code(500);
    echo "ERROR: DB-forbindelse fejlede: " . $e->getMessage() . "\n";
    echo "Tjek DATABASE_URL, brugernavn og password i .env.\n";
    exit;
}

try {
    $count = (int) $pdo->query('SELECT COUNT(*) FROM admin_tokens')->fetchColumn();
} catch (\Throwable $e) {
    http_response_code(500);
    echo "ERROR: kunne ikke læse admin_tokens-tabellen: " . $e->getMessage() . "\n";
    echo "Er schema-migrationerne kørt? Se server/schema/README.md.\n";
    exit;
}

if ($count > 0) {
    http_response_code(403);
    echo "Setup er allerede kørt — der findes $count admin-token(s) i databasen.\n\n";
    echo "SLET denne fil (server/public/setup.php) nu — den er ikke længere nødvendig.\n";
    exit;
}

try {
    $token = TokenHasher::generate();
    $hash  = TokenHasher::hash($token);
    $stmt  = $pdo->prepare('INSERT INTO admin_tokens (token_hash) VALUES (:hash)');
    $stmt->execute(['hash' => $hash]);
} catch (\Throwable $e) {
    http_response_code(500);
    echo "ERROR: kunne ikke oprette admin-token: " . $e->getMessage() . "\n";
    exit;
}

echo "=== KeePass Delta-Sync — Setup ===\n\n";
echo "Admin-token oprettet. Gem den NU — den vises ikke igen:\n\n";
echo "    $token\n\n";
echo "Brug i HTTP-headeren: Authorization: Bearer <token>\n\n";
echo str_repeat('=', 64) . "\n";
echo " VIGTIGT: SLET filen server/public/setup.php nu.\n";
echo " Guarden ovenfor forhindrer dobbelt-kørsel, men filen er stadig\n";
echo " unødvendig angrebs-flade så længe den ligger i web-roden.\n";
echo str_repeat('=', 64) . "\n";
