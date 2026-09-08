<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Controllers\Admin;

use KeePassDeltaSync\Audit\AuditLogger;
use KeePassDeltaSync\Audit\EventType;
use KeePassDeltaSync\Auth\AdminAccount;
use KeePassDeltaSync\Auth\AdminSession;
use KeePassDeltaSync\Auth\AuthContext;
use KeePassDeltaSync\Config;
use KeePassDeltaSync\Http\JsonResponse;
use KeePassDeltaSync\Http\Request;
use KeePassDeltaSync\Http\Response;
use PDO;

/**
 * Login og logout for admin-panelet:
 *   POST   /api/v1/admin/session   — brugernavn + adgangskode → session-cookie
 *   DELETE /api/v1/admin/session   — log ud, slet sessionen server-side
 *
 * Begge er routet som 'public', fordi de netop er vejen TIL legitimation.
 * Beskyttelsen er rate limiting, ikke et token.
 */
final class SessionController
{
    public function __construct(
        private readonly PDO    $pdo,
        private readonly Config $config,
    ) {}

    /**
     * @param array<string,string> $params
     */
    public function create(Request $req, array $params, ?AuthContext $auth, AuditLogger $log): Response
    {
        $ip = $req->clientIp($this->config->trustedProxies);

        if (!AdminAccount::exists($this->pdo)) {
            // Ingen konto konfigureret. Sig det ligeud: det er en
            // opsætningsfejl, ikke et forkert kodeord, og at skjule det ville
            // kun sende folk på jagt efter en adgangskode der ikke findes.
            return new JsonResponse(503, [
                'error'   => 'admin_account_not_configured',
                'message' => 'no admin account exists — set ADMIN_USERNAME and ADMIN_PASSWORD, '
                    . 'or run: php bin/admin admin:set-password',
            ]);
        }

        if (\KeePassDeltaSync\Auth\RateLimiter::tooManyAttempts(
            $this->pdo, $ip, $this->config->rateLimitAuthPerMinute
        )) {
            $log->info(EventType::AuthRateLimited, [
                'details' => ['route' => '/api/v1/admin/session'],
                'success' => false,
            ]);
            return new JsonResponse(429, [
                'error'   => 'too_many_attempts',
                'message' => 'too many login attempts — wait a minute and try again',
            ], ['Retry-After' => '60']);
        }

        $body     = $req->jsonBody();
        $username = is_string($body['username'] ?? null) ? trim($body['username']) : '';
        $password = is_string($body['password'] ?? null) ? $body['password'] : '';

        if ($username === '' || $password === '') {
            \KeePassDeltaSync\Auth\RateLimiter::record($this->pdo, $ip);
            return new JsonResponse(400, [
                'error'   => 'invalid_request',
                'message' => 'username and password are required',
            ]);
        }

        if (!AdminAccount::verify($this->pdo, $username, $password, $this->config)) {
            \KeePassDeltaSync\Auth\RateLimiter::record($this->pdo, $ip);
            $log->info(EventType::AuthFailure, [
                'details' => ['route' => '/api/v1/admin/session', 'reason' => 'bad_credentials'],
                'success' => false,
            ]);
            // Samme svar uanset om det var brugernavnet eller kodeordet.
            return new JsonResponse(401, [
                'error'   => 'unauthorized',
                'message' => 'invalid username or password',
            ]);
        }

        \KeePassDeltaSync\Auth\RateLimiter::clear($this->pdo, $ip);

        $token = AdminSession::create($this->pdo, $this->config->adminSessionTtlHours);

        $log->info(EventType::AuthSuccess, [
            'details' => ['route' => '/api/v1/admin/session', 'method' => 'password'],
        ]);

        return new JsonResponse(
            200,
            ['expires_in' => $this->config->adminSessionTtlHours * 3600],
            ['Set-Cookie' => $this->cookie($token, $req->isSecure($this->config->trustedProxies))],
        );
    }

    /**
     * @param array<string,string> $params
     */
    public function destroy(Request $req, array $params, ?AuthContext $auth, AuditLogger $log): Response
    {
        $token = $req->cookie(AdminSession::COOKIE_NAME);
        if ($token !== null) {
            AdminSession::destroy($this->pdo, $token);
        }

        // Svarer 204 uanset om der var en session. Logout skal ikke kunne
        // bruges til at afgøre om en cookie var gyldig.
        return new Response(
            204,
            ['Set-Cookie' => $this->expiredCookie($req->isSecure($this->config->trustedProxies))],
            '',
        );
    }

    /**
     * Path=/ frem for APP_BASE_PATH: admin.html serveres fra web-roden af
     * Apache uanset base path (den er en statisk fil, ikke en route), så en
     * sti-begrænset cookie ville ikke følge med panelets kald.
     */
    private function cookie(string $token, bool $secure): string
    {
        $parts = [
            AdminSession::COOKIE_NAME . '=' . $token,
            'Path=/',
            'Max-Age=' . ($this->config->adminSessionTtlHours * 3600),
            'HttpOnly',
            'SameSite=Strict',
        ];
        if ($secure) {
            $parts[] = 'Secure';
        }
        return implode('; ', $parts);
    }

    private function expiredCookie(bool $secure): string
    {
        $parts = [
            AdminSession::COOKIE_NAME . '=',
            'Path=/',
            'Max-Age=0',
            'HttpOnly',
            'SameSite=Strict',
        ];
        if ($secure) {
            $parts[] = 'Secure';
        }
        return implode('; ', $parts);
    }
}
