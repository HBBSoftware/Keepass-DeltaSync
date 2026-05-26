<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync;

/**
 * Route-tabel for /api/v1/.
 *
 * Endpoint-listen følger spec § "API-endpoints". Controllers er endnu ikke
 * implementeret — hver route peger på en `Controller::method`-streng som
 * senere bliver opløst via en simpel controller-dispatcher.
 *
 * Auth-typer:
 *   - 'device'     : enhedstoken (default for bruger-endpoints)
 *   - 'enrollment' : engangs-enrollment-token
 *   - 'admin'      : admin-token (kan ikke læse blob-indhold)
 */
final class Router
{
    /** @var list<array{method:string, path:string, handler:string, auth:string}> */
    private array $routes = [];

    public function registerRoutes(): void
    {
        // --- Enrollment (engangstoken) ---
        $this->add('POST', '/api/v1/devices/enroll', 'EnrollmentController::enroll', 'enrollment');

        // --- Bruger-endpoints (enhedstoken) ---
        $this->add('GET',    '/api/v1/me',                                            'MeController::show',          'device');

        $this->add('POST',   '/api/v1/databases',                                     'DatabaseController::create',  'device');
        $this->add('GET',    '/api/v1/databases',                                     'DatabaseController::index',   'device');
        $this->add('DELETE', '/api/v1/databases/{id}',                                'DatabaseController::destroy', 'device');

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

    /** @return list<array{method:string, path:string, handler:string, auth:string}> */
    public function routes(): array
    {
        return $this->routes;
    }

    public function dispatch(): never
    {
        // TODO: route-matching + auth-middleware + controller-invocation.
        // Indtil videre returneres 501 så vi tydeligt kan se at serveren kører,
        // men endnu ikke håndterer requests.
        http_response_code(501);
        header('Content-Type: application/json');
        echo json_encode([
            'error'   => 'not_implemented',
            'message' => 'Server skeleton — endpoints not yet wired.',
        ]);
        exit;
    }

    private function add(string $method, string $path, string $handler, string $auth): void
    {
        $this->routes[] = compact('method', 'path', 'handler', 'auth');
    }
}
