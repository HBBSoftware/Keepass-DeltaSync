<?php
// SPDX-License-Identifier: AGPL-3.0-or-later

declare(strict_types=1);

namespace KeePassDeltaSync\Audit;

/**
 * Alle event-typer fra spec § "Audit-log → Event-typer".
 *
 * INFO-niveau (logges altid):  auth.*, enrollment.*, device.revoked, user.*, admin.action
 * DEBUG-niveau (kun i udv.):   database.*, entries.*, entry.*
 */
enum EventType: string
{
    // --- Auth & user lifecycle (INFO) ---
    case AuthSuccess       = 'auth.success';
    case AuthFailure       = 'auth.failure';
    case AuthRateLimited   = 'auth.rate_limited';
    case EnrollmentSuccess = 'enrollment.success';
    case EnrollmentFailure = 'enrollment.failure';
    case DeviceRevoked     = 'device.revoked';
    case UserCreated       = 'user.created';
    case UserDisabled      = 'user.disabled';
    case UserDeleted       = 'user.deleted';
    case AdminAction       = 'admin.action';

    // --- Activity (DEBUG) ---
    case DatabaseCreated       = 'database.created';
    case DatabaseDeleted       = 'database.deleted';
    case EntriesChangesFetched = 'entries.changes_fetched';
    case EntryPut              = 'entry.put';
    case EntryDeleted          = 'entry.deleted';
    case EntryVersionsListed   = 'entry.versions_listed';
    case EntryRestored         = 'entry.restored';
    case EntryVersionFetched   = 'entry.version_fetched';
    case GroupPut              = 'group.put';
    case GroupDeleted          = 'group.deleted';
}
