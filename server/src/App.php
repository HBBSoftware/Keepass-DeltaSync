<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync;

use KeePassDeltaSync\Audit\AuditLogger;
use KeePassDeltaSync\Audit\Cleanup;
use KeePassDeltaSync\Audit\LogLevel;
use KeePassDeltaSync\Auth\TokenAuthenticator;
use KeePassDeltaSync\Db\Connection;
use KeePassDeltaSync\Http\HttpException;
use KeePassDeltaSync\Http\JsonResponse;
use KeePassDeltaSync\Http\Request;

/**
 * Applikationsbootstrap. Læser konfiguration, registrerer ruter, og
 * dispatcher den indkommende HTTP-request.
 *
 * Fejl-håndtering:
 *   - HttpException → respons med den specificerede status + JSON-body
 *   - Alt andet     → logges + 500 (ingen interne detaljer i body'en)
 */
final class App
{
    public function __construct(
        private readonly Config $config,
        private readonly Router $router,
    ) {}

    public static function bootstrap(string $rootDir): self
    {
        $config = Config::loadFromEnv($rootDir);
        $router = new Router();
        $router->registerRoutes();

        return new self($config, $router);
    }

    public function handle(): void
    {
        $request = Request::fromGlobals();

        try {
            $pdo = Connection::fromConfig($this->config);

            // Best-effort audit-/enrollment-oprydning. Throttled til 1×/time via
            // system_state. Aldrig blokerende — fejl swallow'es, hovedrequesten
            // må ikke fejle pga. oprydning.
            try {
                Cleanup::runIfDue($pdo, $this->config->auditRetentionDays);
            } catch (\Throwable $e) {
                error_log('[cleanup] startup-trigger failed: ' . $e->getMessage());
            }

            $authenticator = new TokenAuthenticator($pdo);
            $logger        = new AuditLogger($pdo, LogLevel::fromConfig($this->config->logLevel));
            $response      = $this->router->dispatch($request, $pdo, $this->config, $authenticator, $logger);
        } catch (HttpException $e) {
            $response = new JsonResponse($e->status, [
                'error'   => $e->errorCode ?? ('error_' . $e->status),
                'message' => $e->getMessage(),
            ]);
        } catch (\Throwable $e) {
            // Internt detaljer logges, men returneres ikke til klienten.
            error_log(sprintf(
                '[keepass-deltasync] unhandled %s: %s at %s:%d',
                $e::class,
                $e->getMessage(),
                $e->getFile(),
                $e->getLine(),
            ));
            $response = new JsonResponse(500, [
                'error'   => 'internal_error',
                'message' => 'an unexpected error occurred',
            ]);
        }

        $response->send();
    }
}
