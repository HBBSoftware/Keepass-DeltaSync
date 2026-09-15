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

        $envUser      = $this->config->adminUsername;
        $envPassword  = $this->config->adminPassword;
        $envConfigured = $envUser !== '' && $envPassword !== '';

        if (!AdminAccount::exists($this->pdo)) {
            // Første login på en vært uden shell. I containeren har
            // docker-entrypoint.sh allerede kaldt admin:ensure, men en
            // filbaseret vært har ingen opstart at hænge det på, og ingen vej
            // til bin/admin. Uden dette var eneste udvej en INSERT i hånden
            // med en Argon2id-hash — og fejlbeskeden nedenfor lovede i
            // forvejen at de to variabler virkede.
            if (!$envConfigured) {
                // Sig det ligeud: det er en opsætningsfejl, ikke et forkert
                // kodeord, og at skjule det ville kun sende folk på jagt efter
                // en adgangskode der ikke findes.
                return new JsonResponse(503, [
                    'error'   => 'admin_account_not_configured',
                    'message' => 'no admin account exists — set ADMIN_USERNAME and ADMIN_PASSWORD '
                        . 'in the environment (or .env), or run: php bin/admin admin:set-password',
                ]);
            }
            AdminAccount::set($this->pdo, $envUser, $envPassword, $this->config);
            $log->info(EventType::AdminAction, [
                'details' => ['action' => 'admin_account_bootstrapped_from_env', 'username' => $envUser],
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
            // Rotation: environment er kilden. Passer det indtastede på det
            // .env siger, men ikke på det databasen har, så er adgangskoden
            // skiftet i filen siden kontoen blev oprettet — og uden shell
            // findes der ingen anden vej til at skifte den.
            //
            // Det koster intet i det normale tilfælde: grenen nås kun når et
            // login allerede er slået fejl, og prøven er to
            // strengsammenligninger, ikke en Argon2-beregning. En angriber
            // vinder heller intet, for at nå hertil skal man i forvejen kende
            // adgangskoden fra .env.
            $matchesEnv = $envConfigured
                && hash_equals($envUser, $username)
                && hash_equals($envPassword, $password);

            if (!$matchesEnv) {
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

            AdminAccount::set($this->pdo, $envUser, $envPassword, $this->config);
            $log->info(EventType::AdminAction, [
                'details' => ['action' => 'admin_account_updated_from_env', 'username' => $envUser],
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
