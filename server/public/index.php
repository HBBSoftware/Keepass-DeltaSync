<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

// Find server-app rod-mappen (= mappen der indeholder bootstrap.php, src/, .env).
//
// Default: én mappe op fra denne fil (= standard layout hvor public/ ligger
// inde i server/).
//
// $APP_ROOT-env-var overrider: brug det hvis index.php deployes ANDET STED end
// inde i server/public/ — fx for at deploye API under en sub-path mens
// hjemmesiden bor på samme web-root. Sæt enten via .htaccess (`SetEnv APP_ROOT
// /full/path/to/server`) eller fastcgi-env. Se docs/deployment.md.
$appRoot = getenv('APP_ROOT');
if ($appRoot === false || $appRoot === '') {
    $appRoot = __DIR__ . '/..';
}
$appRoot = rtrim($appRoot, '/');

require $appRoot . '/bootstrap.php';

KeePassDeltaSync\App::bootstrap($appRoot)->handle();
