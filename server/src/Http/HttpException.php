<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Http;

/**
 * Kastes fra controllere for at returnere en bestemt HTTP-status. Fanges
 * centralt i App::handle og konverteres til en JsonResponse, så controllere
 * kan skrive `throw new HttpException(404, ...)` i stedet for at konstruere
 * en respons manuelt.
 *
 * $errorCode er valgfri maskinlæsbar identifier (fx 'not_found', 'invalid_body').
 * Hvis null, bruges en kode udledt af statussen ('error_404' osv.).
 */
class HttpException extends \RuntimeException
{
    public function __construct(
        public readonly int     $status,
        string                  $message    = '',
        public readonly ?string $errorCode  = null,
    ) {
        parent::__construct($message);
    }
}
