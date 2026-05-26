<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Http;

/**
 * Immutable wrapper omkring den indkommende HTTP-request.
 *
 * Headers er normaliseret til lowercase keys, så caller'en kan kigge dem op
 * uden at bekymre sig om "Authorization" vs. "AUTHORIZATION" vs. "authorization".
 */
final class Request
{
    /**
     * @param array<string, string>      $headers lowercase header-navne → værdi
     * @param array<string, string|array> $query   $_GET
     */
    public function __construct(
        public readonly string $method,
        public readonly string $path,
        public readonly array  $headers,
        public readonly array  $query,
        public readonly string $body,
    ) {}

    public static function fromGlobals(): self
    {
        $method = strtoupper((string) ($_SERVER['REQUEST_METHOD'] ?? 'GET'));
        $uri    = (string) ($_SERVER['REQUEST_URI'] ?? '/');
        $path   = parse_url($uri, PHP_URL_PATH) ?: '/';

        $headers = [];
        foreach ($_SERVER as $key => $value) {
            if (is_string($key) && str_starts_with($key, 'HTTP_')) {
                $name = strtolower(str_replace('_', '-', substr($key, 5)));
                $headers[$name] = (string) $value;
            }
        }
        // CONTENT_TYPE / CONTENT_LENGTH er ikke prefixet med HTTP_ i $_SERVER.
        if (isset($_SERVER['CONTENT_TYPE'])) {
            $headers['content-type'] = (string) $_SERVER['CONTENT_TYPE'];
        }
        if (isset($_SERVER['CONTENT_LENGTH'])) {
            $headers['content-length'] = (string) $_SERVER['CONTENT_LENGTH'];
        }

        $body = file_get_contents('php://input');
        if ($body === false) {
            $body = '';
        }

        return new self($method, $path, $headers, $_GET ?? [], $body);
    }

    /** Ekstraher bearer-tokenet fra Authorization-headeren, eller null. */
    public function bearerToken(): ?string
    {
        $auth = $this->headers['authorization'] ?? '';
        if (!preg_match('/^Bearer\s+(.+)$/i', $auth, $m)) {
            return null;
        }
        return trim($m[1]);
    }

    /**
     * Parser body'en som JSON. Returnerer [] for tom body.
     * Kaster HttpException(400) hvis body'en findes men ikke er gyldig JSON.
     */
    public function jsonBody(): array
    {
        if ($this->body === '') {
            return [];
        }
        $decoded = json_decode($this->body, true);
        if (!is_array($decoded)) {
            throw new HttpException(400, 'invalid json body');
        }
        return $decoded;
    }

    /**
     * Klient-IP for audit-log. Bag reverse proxy bør X-Forwarded-For honoreres,
     * men det kræver konfiguration af hvilke proxy-IP'er der er betroede —
     * TODO i en senere milestone.
     */
    public function clientIp(): ?string
    {
        $ip = $_SERVER['REMOTE_ADDR'] ?? null;
        return is_string($ip) && $ip !== '' ? $ip : null;
    }

    public function userAgent(): ?string
    {
        return $this->headers['user-agent'] ?? null;
    }
}
