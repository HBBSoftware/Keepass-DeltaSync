<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

/**
 * Bootstrap-autoloader. Composer-optional.
 *
 * Projektet har ingen tredjeparts-deps (kun PHP-extensions), så vi behøver
 * ikke Composer eller en `vendor/`-mappe i produktion. Hvis Composer alligevel
 * er kørt (typisk i udvikling, eller hvis en hosting-platform installerer
 * automatisk), foretrækker vi dens autoloader — den giver class-map cache
 * og er bedre optimeret. Ellers falder vi tilbage til en lille PSR-4
 * implementation der mapper `KeePassDeltaSync\X\Y` → `src/X/Y.php`.
 */

$composerAutoload = __DIR__ . '/vendor/autoload.php';
if (is_file($composerAutoload)) {
    require $composerAutoload;
    return;
}

spl_autoload_register(static function (string $class): void {
    $prefix = 'KeePassDeltaSync\\';
    if (!str_starts_with($class, $prefix)) {
        return;
    }
    $relative = substr($class, strlen($prefix));
    $file     = __DIR__ . '/src/' . str_replace('\\', '/', $relative) . '.php';
    if (is_file($file)) {
        require $file;
    }
});
