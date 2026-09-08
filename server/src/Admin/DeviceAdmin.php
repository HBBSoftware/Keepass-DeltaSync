<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Admin;

use PDO;

/**
 * Read-only enheds-oversigt til admin.
 *
 * Bevidst uden skrive-operationer. En admin kan i forvejen læse audit-loggen
 * med device_id'er, oprette brugere og udstede enrollment-tokens, så en liste
 * giver ingen ny magt — den gør bare data læselige, der allerede er synlige
 * som rå UUID'er. At tilbagekalde en enhed er derimod en *ny* rettighed og
 * hører til en selvstændig beslutning.
 *
 * devices.token_hash forlader ALDRIG dette lag. Hashen kan ikke bruges til at
 * autentificere (serveren sammenligner den mod hashen af det fremsendte
 * token), men den er nøglen i opslaget, og der er ingen grund til at flytte
 * den nærmere en HTTP-respons end nødvendigt. Kolonnerne listes derfor
 * eksplicit — aldrig SELECT *.
 */
final class DeviceAdmin
{
    public function __construct(private readonly PDO $pdo) {}

    /**
     * Alle enheder med ejer, sorteret så de længst inaktive falder i øjnene.
     *
     * @param  string|null $userId Afgræns til én bruger. Antages valideret som UUID af kald-laget.
     * @return list<array{
     *     id:string, name:?string, user_id:string, username:string,
     *     enrolled_at:?string, last_seen:?string, has_public_key:bool
     * }>
     */
    public function listDevices(?string $userId = null): array
    {
        $sql = 'SELECT d.id, d.name, d.user_id, u.username,
                       d.enrolled_at, d.last_seen,
                       (d.public_key IS NOT NULL) AS has_public_key
                  FROM devices d
                  JOIN users u ON u.id = d.user_id';
        $args = [];
        if ($userId !== null) {
            $sql .= ' WHERE d.user_id = :uid';
            $args['uid'] = $userId;
        }
        // NULLS FIRST: en enhed der aldrig har været i kontakt er mere
        // interessant end en der var her i går.
        $sql .= ' ORDER BY d.last_seen DESC NULLS FIRST, u.username';

        $stmt = $this->pdo->prepare($sql);
        $stmt->execute($args);

        return array_map(static fn(array $r): array => [
            'id'             => (string) $r['id'],
            'name'           => $r['name'] !== null ? (string) $r['name'] : null,
            'user_id'        => (string) $r['user_id'],
            'username'       => (string) $r['username'],
            'enrolled_at'    => self::isoUtc($r['enrolled_at']),
            'last_seen'      => self::isoUtc($r['last_seen']),
            'has_public_key' => (bool) $r['has_public_key'],
        ], $stmt->fetchAll());
    }

    private static function isoUtc(mixed $value): ?string
    {
        if (!is_string($value) || $value === '') {
            return null;
        }
        return (new \DateTimeImmutable($value))
            ->setTimezone(new \DateTimeZone('UTC'))
            ->format('Y-m-d\TH:i:s\Z');
    }
}
