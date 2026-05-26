<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Auth;

/**
 * Kastes når et bearer-token ikke kan valideres (ukendt, udløbet, brugt,
 * forkert type, eller tilhørende en deaktiveret bruger). Indeholder bevidst
 * ikke detaljer om hvorfor — den der kalder skal returnere en generisk 401
 * udadtil, så vi ikke lækker information til en angriber.
 */
final class AuthenticationException extends \RuntimeException
{
}
