<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Http;

/**
 * En simpel HTTP-respons. Ikke final, så JsonResponse kan extende.
 *
 * @phpstan-consistent-constructor
 */
class Response
{
    /** @param array<string, string> $headers */
    public function __construct(
        public readonly int    $status,
        public readonly array  $headers,
        public readonly string $body,
    ) {}

    public function send(): void
    {
        http_response_code($this->status);
        foreach ($this->headers as $name => $value) {
            header($name . ': ' . $value, true);
        }
        echo $this->body;
    }
}
