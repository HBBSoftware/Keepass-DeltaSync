<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync;

/**
 * Applikationsbootstrap. Læser konfiguration, registrerer ruter, og dispatcher
 * den indkommende HTTP-request.
 *
 * Skelet: dispatch returnerer endnu en 501.
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
        // TODO: best-effort audit-cleanup ved opstart.
        // Spec: pg_try_advisory_lock(42) + 1-times throttle via system_state.last_cleanup_at.

        $this->router->dispatch();
    }
}
