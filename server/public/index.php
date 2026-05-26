<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

require __DIR__ . '/../vendor/autoload.php';

KeePassDeltaSync\App::bootstrap(__DIR__ . '/..')->handle();
