<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync;

use KeePassDeltaSync\Audit\AuditLogger;
use KeePassDeltaSync\Audit\EventType;
use KeePassDeltaSync\Auth\AuthenticationException;
use KeePassDeltaSync\Auth\TokenAuthenticator;
use KeePassDeltaSync\Auth\TokenType;
use KeePassDeltaSync\Config;
use KeePassDeltaSync\Http\HttpException;
use KeePassDeltaSync\Http\JsonResponse;
use KeePassDeltaSync\Http\Request;
use KeePassDeltaSync\Http\Response;
use PDO;

/**
 * Route-tabel + dispatcher for /api/v1/.
 *
 * Routes registreres med {param}-placeholders som omsættes til regex og giver
 * navngivne capture-grupper. Eksempel: '/api/v1/databases/{id}/entries/{uuid}'
 * → '#^/api/v1/databases/(?P<id>[^/]+)/entries/(?P<uuid>[^/]+)$#'.
 *
 * Hver route har en required token-type ('admin' | 'enrollment' | 'device').
 * TokenAuthenticator validerer KUN mod den relevante tabel, så en admin-token
 * kan ikke smutte ind på et bruger-endpoint eller omvendt.
 *
 * 404 før auth: hvis ruten ikke findes (vilkårligt path under /api/v1/), så
 * returnerer dispatcheren 404 uden at forsøge auth — det er den offentlige
 * URL-flade og afslører ikke noget hemmeligt. 404 for "fremmed bruger's
 * ressource" håndteres derimod af den enkelte controller efter auth, så
 * vi opretholder 404-not-403-reglen fra spec'en uden info-leak.
 */
final class Router
{
    /** @var list<array{method:string, path:string, handler:string, auth:string, regex:string}> */
    private array $routes = [];

    public function registerRoutes(): void
    {
        // --- Enrollment (engangstoken) ---
        $this->add('POST', '/api/v1/devices/enroll', 'EnrollmentController::enroll', 'enrollment');

        // --- Bruger-endpoints (enhedstoken) ---
        $this->add('GET',    '/api/v1/me',                                            'MeController::show',          'device');
        $this->add('PATCH',  '/api/v1/me',                                            'MeController::update',        'device');

        $this->add('POST',   '/api/v1/databases',                                     'DatabaseController::create',  'device');
        $this->add('GET',    '/api/v1/databases',                                     'DatabaseController::index',   'device');
        $this->add('DELETE', '/api/v1/databases/{id}',                                'DatabaseController::destroy', 'device');

        $this->add('GET',    '/api/v1/users/lookup',                                  'ShareController::lookupUser', 'device');
        $this->add('GET',    '/api/v1/databases/{id}/shares',                         'ShareController::index',      'device');
        $this->add('POST',   '/api/v1/databases/{id}/shares',                         'ShareController::create',     'device');
        $this->add('DELETE', '/api/v1/databases/{id}/shares/{user_id}',               'ShareController::destroy',    'device');

        $this->add('GET',    '/api/v1/databases/{id}/changes',                        'EntryController::changes',    'device');
        $this->add('PUT',    '/api/v1/databases/{id}/entries/{uuid}',                 'EntryController::put',        'device');
        $this->add('DELETE', '/api/v1/databases/{id}/entries/{uuid}',                 'EntryController::destroy',    'device');
        $this->add('GET',    '/api/v1/databases/{id}/entries/{uuid}/versions',        'EntryController::versions',   'device');
        $this->add('GET',    '/api/v1/databases/{id}/entries/{uuid}/versions/{num}',  'EntryController::version',    'device');
        $this->add('POST',   '/api/v1/databases/{id}/entries/{uuid}/restore/{num}',   'EntryController::restore',    'device');

        $this->add('GET',    '/api/v1/devices',                                       'DeviceController::index',     'device');
        $this->add('DELETE', '/api/v1/devices/{id}',                                  'DeviceController::destroy',   'device');

        $this->add('GET',    '/api/v1/log',                                           'LogController::index',        'device');

        // --- Admin-endpoints (admin-token; ingen adgang til entry-blobs) ---
        $this->add('POST',   '/api/v1/admin/users',                                              'Admin\\UserController::create',          'admin');
        $this->add('GET',    '/api/v1/admin/users',                                              'Admin\\UserController::index',           'admin');
        $this->add('PATCH',  '/api/v1/admin/users/{id}',                                         'Admin\\UserController::update',          'admin');
        $this->add('DELETE', '/api/v1/admin/users/{id}',                                         'Admin\\UserController::destroy',         'admin');
        $this->add('POST',   '/api/v1/admin/users/{id}/enrollment',                              'Admin\\UserController::enrollment',      'admin');
        $this->add('POST',   '/api/v1/admin/databases/{id}/entries/{uuid}/restore/{num}',        'Admin\\EntryRestoreController::restore', 'admin');
        $this->add('GET',    '/api/v1/admin/log',                                                'Admin\\LogController::index',            'admin');
        $this->add('POST',   '/api/v1/admin/log/cleanup',                                        'Admin\\LogController::cleanup',          'admin');
    }

    public function dispatch(
        Request            $request,
        PDO                $pdo,
        Config             $config,
        TokenAuthenticator $authenticator,
        AuditLogger        $logger,
    ): Response {
        $match = $this->findRoute($request->method, $request->path);
        if ($match === null) {
            return new JsonResponse(404, [
                'error'   => 'not_found',
                'message' => 'route not found',
            ]);
        }

        [$route, $params] = $match;

        $bearer = $request->bearerToken();
        if ($bearer === null) {
            $logger->forRequest($request)->info(EventType::AuthFailure, [
                'details' => ['route' => $route['path'], 'reason' => 'missing_bearer'],
                'success' => false,
            ]);
            return new JsonResponse(401, [
                'error'   => 'unauthorized',
                'message' => 'missing bearer token',
            ]);
        }

        try {
            $ctx = $authenticator->authenticate($bearer, TokenType::from($route['auth']));
        } catch (AuthenticationException) {
            $logger->forRequest($request)->info(EventType::AuthFailure, [
                'details' => ['route' => $route['path'], 'reason' => 'invalid_token'],
                'success' => false,
            ]);
            return new JsonResponse(401, [
                'error'   => 'unauthorized',
                'message' => 'invalid token',
            ]);
        }

        $requestLogger = $logger->forRequest($request, $ctx);
        $requestLogger->info(EventType::AuthSuccess, [
            'details' => ['route' => $route['path'], 'method' => $request->method],
        ]);

        // Instantiér controller. Handler-strengen 'MeController::show' bliver
        // til KeePassDeltaSync\Controllers\MeController::show. Sub-namespaces
        // (fx 'Admin\\UserController::create') bevares.
        [$class, $method] = explode('::', $route['handler'], 2);
        $fqcn = 'KeePassDeltaSync\\Controllers\\' . $class;

        if (!class_exists($fqcn)) {
            // Controlleren er routet men ikke implementeret endnu.
            return new JsonResponse(501, [
                'error'   => 'not_implemented',
                'message' => $fqcn . ' not yet implemented',
            ]);
        }

        $controller = new $fqcn($pdo, $config);
        if (!method_exists($controller, $method)) {
            throw new HttpException(500, "handler method $fqcn::$method missing");
        }

        /** @var Response */
        return $controller->$method($request, $params, $ctx, $requestLogger);
    }

    /**
     * Find første route der matcher method + path.
     *
     * @return array{0: array{method:string, path:string, handler:string, auth:string, regex:string}, 1: array<string,string>}|null
     */
    private function findRoute(string $method, string $path): ?array
    {
        foreach ($this->routes as $route) {
            if ($route['method'] !== $method) {
                continue;
            }
            if (preg_match($route['regex'], $path, $matches)) {
                $params = [];
                foreach ($matches as $key => $value) {
                    if (is_string($key)) {
                        $params[$key] = $value;
                    }
                }
                return [$route, $params];
            }
        }
        return null;
    }

    private function add(string $method, string $path, string $handler, string $auth): void
    {
        $regex = preg_replace_callback(
            '/\{(\w+)\}/',
            static fn(array $m): string => '(?P<' . $m[1] . '>[^/]+)',
            $path,
        );
        $this->routes[] = [
            'method'  => $method,
            'path'    => $path,
            'handler' => $handler,
            'auth'    => $auth,
            'regex'   => '#^' . $regex . '$#',
        ];
    }

    /** @return list<array{method:string, path:string, handler:string, auth:string, regex:string}> */
    public function routes(): array
    {
        return $this->routes;
    }
}
