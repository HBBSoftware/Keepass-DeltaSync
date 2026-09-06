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
     * @param array<string, string>       $cookies $_COOKIE
     */
    public function __construct(
        public readonly string $method,
        public readonly string $path,
        public readonly array  $headers,
        public readonly array  $query,
        public readonly string $body,
        public readonly array  $cookies = [],
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

        return new self($method, $path, $headers, $_GET ?? [], $body, $_COOKIE ?? []);
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

    /** Værdien af en cookie, eller null hvis den ikke er sat. */
    public function cookie(string $name): ?string
    {
        $value = $this->cookies[$name] ?? null;
        return is_string($value) && $value !== '' ? $value : null;
    }

    /**
     * Er forbindelsen krypteret hele vejen frem til klienten?
     *
     * Afgør om session-cookien får Secure-flaget. Serveren selv taler altid
     * almindelig HTTP, så bag en TLS-terminerende proxy kan svaret kun komme
     * fra X-Forwarded-Proto — og den header kan enhver klient sætte. Derfor
     * honoreres den KUN når requesten kommer fra en IP i TRUSTED_PROXIES.
     * Uden den konfiguration ser vi på den faktiske forbindelse, hvilket på et
     * LAN over HTTP giver false, og cookien sættes uden Secure. Det er den
     * rigtige afvejning: et Secure-flag over HTTP ville betyde at browseren
     * smider cookien væk, og så kunne ingen logge ind overhovedet.
     *
     * @param list<string> $trustedProxies
     */
    public function isSecure(array $trustedProxies = []): bool
    {
        if ($this->fromTrustedProxy($trustedProxies)) {
            $proto = strtolower(trim($this->headers['x-forwarded-proto'] ?? ''));
            if ($proto !== '') {
                // Ved flere hop er den yderste (klient-nære) værdi den første.
                $first = trim(explode(',', $proto)[0]);
                return $first === 'https';
            }
        }

        $https = $_SERVER['HTTPS'] ?? '';
        if (is_string($https) && $https !== '' && strtolower($https) !== 'off') {
            return true;
        }
        return (int) ($_SERVER['SERVER_PORT'] ?? 0) === 443;
    }

    /**
     * Klient-IP for audit-log og rate limiting.
     *
     * X-Forwarded-For honoreres kun fra betroede proxier — ellers kunne
     * enhver klient sætte headeren og dermed vælge sin egen rate-limit-bucket.
     *
     * @param list<string> $trustedProxies
     */
    public function clientIp(array $trustedProxies = []): ?string
    {
        if ($this->fromTrustedProxy($trustedProxies)) {
            $forwarded = trim($this->headers['x-forwarded-for'] ?? '');
            if ($forwarded !== '') {
                $first = trim(explode(',', $forwarded)[0]);
                if ($first !== '' && filter_var($first, FILTER_VALIDATE_IP) !== false) {
                    return $first;
                }
            }
        }

        $ip = $_SERVER['REMOTE_ADDR'] ?? null;
        return is_string($ip) && $ip !== '' ? $ip : null;
    }

    /** @param list<string> $trustedProxies IP'er eller CIDR'er. */
    private function fromTrustedProxy(array $trustedProxies): bool
    {
        if ($trustedProxies === []) {
            return false;
        }
        $remote = $_SERVER['REMOTE_ADDR'] ?? '';
        if (!is_string($remote) || $remote === '') {
            return false;
        }
        foreach ($trustedProxies as $trusted) {
            if (self::ipMatches($remote, $trusted)) {
                return true;
            }
        }
        return false;
    }

    /** Matcher en IP mod en enkelt IP eller en CIDR-blok (IPv4 og IPv6). */
    private static function ipMatches(string $ip, string $pattern): bool
    {
        if (!str_contains($pattern, '/')) {
            return $ip === $pattern;
        }

        [$subnet, $bitsRaw] = explode('/', $pattern, 2);
        $bits = (int) $bitsRaw;

        $ipBin     = @inet_pton($ip);
        $subnetBin = @inet_pton($subnet);
        if ($ipBin === false || $subnetBin === false || strlen($ipBin) !== strlen($subnetBin)) {
            return false;
        }
        if ($bits < 0 || $bits > strlen($ipBin) * 8) {
            return false;
        }

        $wholeBytes = intdiv($bits, 8);
        $restBits   = $bits % 8;

        if ($wholeBytes > 0 && strncmp($ipBin, $subnetBin, $wholeBytes) !== 0) {
            return false;
        }
        if ($restBits === 0) {
            return true;
        }

        $mask = ~((1 << (8 - $restBits)) - 1) & 0xFF;
        return (ord($ipBin[$wholeBytes]) & $mask) === (ord($subnetBin[$wholeBytes]) & $mask);
    }

    public function userAgent(): ?string
    {
        return $this->headers['user-agent'] ?? null;
    }
}
