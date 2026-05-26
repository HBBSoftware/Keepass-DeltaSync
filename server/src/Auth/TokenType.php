<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Auth;

enum TokenType: string
{
    case Admin      = 'admin';
    case Enrollment = 'enrollment';
    case Device     = 'device';
}
