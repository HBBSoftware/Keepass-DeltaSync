<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Http;

final class JsonResponse extends Response
{
    /** @param array<string, string> $extraHeaders */
    public function __construct(int $status, mixed $data, array $extraHeaders = [])
    {
        $body = json_encode(
            $data,
            JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE | JSON_THROW_ON_ERROR
        );

        parent::__construct(
            $status,
            ['Content-Type' => 'application/json; charset=utf-8'] + $extraHeaders,
            $body,
        );
    }
}
