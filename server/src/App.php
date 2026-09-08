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
    /**
     * Written by the container entrypoint when startup failed. Lives outside
     * public/, so Apache can never serve it directly.
     */
    private const STARTUP_ERROR_FILE = '.startup-error';

    public function __construct(
        private readonly Config $config,
        private readonly Router $router,
        private readonly string $rootDir,
    ) {}

    public static function bootstrap(string $rootDir): self
    {
        $config = Config::loadFromEnv($rootDir);
        $router = new Router();
        $router->registerRoutes();

        return new self($config, $router, $rootDir);
    }

    /**
     * Startet konfigureret forkert? Entrypointet skriver grunden hertil og
     * starter webserveren alligevel, netop så den kan siges højt. Uden det
     * ender en fejlkonfiguration i en crash-loop, hvor browseren kun kan sige
     * "unable to connect" og grunden kun findes i container-loggen.
     */
    private function startupError(): ?string
    {
        $path = $this->rootDir . '/' . self::STARTUP_ERROR_FILE;
        if (!is_readable($path)) {
            return null;
        }
        $reason = trim((string) file_get_contents($path));
        if ($reason === '') {
            return null;
        }
        // Beskeder skrives af os selv, men kap alligevel: filen kan være
        // blevet noget andet end vi tror.
        return mb_substr($reason, 0, 2000);
    }

    public function handle(): void
    {
        $request = Request::fromGlobals();

        $startupError = $this->startupError();
        if ($startupError !== null) {
            // Gælder ALLE ruter, health inklusive. Healthchecket skal blive
            // ved med at fejle — containeren ER ubrugelig — men det skal
            // fejle med en grund i stedet for tavshed.
            (new JsonResponse(503, [
                'error'   => 'startup_error',
                'message' => $startupError,
            ]))->send();
            return;
        }

        try {
            try {
                $pdo = Connection::fromConfig($this->config);
            } catch (\Throwable $e) {
                // Uden det her ender en utilgængelig database som et generisk
                // 500 "an unexpected error occurred" på hver eneste rute —
                // også /health, som ellers lover at rapportere db-status.
                error_log('[keepass-deltasync] database unavailable: ' . $e->getMessage());
                (new JsonResponse(503, [
                    'error'   => 'database_unavailable',
                    'message' => 'the database is not reachable — check that the database '
                        . 'container is running and that DATABASE_* / PG* point at it',
                    'db'      => 'down',
                ]))->send();
                return;
            }

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
