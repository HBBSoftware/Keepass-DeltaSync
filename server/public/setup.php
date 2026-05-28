<?php
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// First-time setup wizard. Walks the admin through:
//
//   Step 1. Configure database connection (writes server/.env)
//   Step 2. Run schema migrations (creates tables)
//   Step 3. Generate the first admin token
//
// Re-runs are blocked once admin_tokens contains at least one row. Delete
// this file after setup is complete — it is not auto-deleted because the
// webserver user typically cannot unlink files inside its own DocumentRoot.
//
// UI is in English by default; ?lang=da switches to Danish.

declare(strict_types=1);

// ------------------------------------------------------------------
// Localization
// ------------------------------------------------------------------

$STRINGS = [
    'en' => [
        'page_title'      => 'DeltaSync Setup',
        'heading'         => 'DeltaSync — First-time setup',
        'subheading'      => 'A short wizard that prepares the server. Run it once, save the admin token, then delete this file.',
        'switch_to'       => 'Dansk',
        'switch_url'      => '?lang=da',

        'guard_done_title'   => 'Setup already complete',
        'guard_done_body'    => 'The admin_tokens table already contains %d entries. This wizard refuses to re-run to avoid accidentally creating extra credentials.',
        'guard_done_action'  => 'Delete this file (server/public/setup.php) — it is no longer needed and adds attack surface.',

        'env_missing'        => 'No .env file found yet. The wizard will create one.',
        'env_unwritable'     => 'Cannot write to .env at %s. Make sure the directory is writable by the webserver user (chmod 700, owned by your user).',
        'bootstrap_missing'  => 'Could not load bootstrap.php (looked at %s). The server code is not in the expected location.',

        'step1_heading'      => 'Step 1 — Database connection',
        'step1_intro'        => 'Enter PostgreSQL connection details. The wizard will test the connection before writing them to server/.env. Leave LOG_LEVEL on INFO unless you know you want DEBUG.',
        'step1_dsn'          => 'PostgreSQL DSN',
        'step1_dsn_hint'     => 'Example: pgsql:host=localhost;port=5432;dbname=keepass_deltasync',
        'step1_user'         => 'Database user',
        'step1_password'     => 'Database password',
        'step1_log'          => 'Log level',
        'step1_audit'        => 'Audit retention (days)',
        'step1_ttl'          => 'Enrollment token TTL (hours)',
        'step1_base'         => 'App base path (optional, e.g. /sync if served under a sub-path)',
        'step1_submit'       => 'Test connection and save .env',

        'step1_ok'           => 'Connection successful. server/.env saved.',
        'step1_fail'         => 'Connection failed: %s',
        'step1_write_fail'   => 'Could not write .env: %s',

        'step2_heading'      => 'Step 2 — Run schema migrations',
        'step2_intro'        => 'The following SQL files will be executed against the database in order. Each one is idempotent in the sense that they fail loudly if their tables already exist — so re-running is safe; it just stops at the first existing table.',
        'step2_no_files'     => 'No schema/*.sql files found at %s.',
        'step2_run'          => 'Run all migrations',
        'step2_ran'          => 'Ran %d migration file(s).',
        'step2_partial'      => 'Migration %s failed: %s. Earlier files may have been applied — check your DB state.',

        'step3_heading'      => 'Step 3 — First admin token',
        'step3_intro'        => 'Generate the initial admin token. This is shown ONCE — copy it before navigating away. You use it with the admin CLI: `keepass-deltasync admin user-create ...`.',
        'step3_run'          => 'Generate admin token',
        'step3_token_label'  => 'Your admin token (save it NOW):',
        'step3_usage'        => 'Use in HTTP requests as: Authorization: Bearer &lt;token&gt;',
        'step3_env_hint'     => 'Or set it as an environment variable for the client CLI:',

        'done_heading'       => 'Setup complete',
        'done_body'          => 'The server is ready. Now do this:',
        'done_step_delete'   => 'Delete <code>server/public/setup.php</code> from the web root. The wizard will refuse to re-run, but the file is unnecessary attack surface.',
        'done_step_user'     => 'Create your first user with the admin CLI:',
        'done_step_share'    => 'Send the resulting enrollment token to that user via a secure channel.',

        'btn_continue'       => 'Continue',
    ],
    'da' => [
        'page_title'      => 'DeltaSync opsætning',
        'heading'         => 'DeltaSync — Førstegangs-opsætning',
        'subheading'      => 'Kort guide der forbereder serveren. Kør den én gang, gem admin-tokenet, og slet denne fil bagefter.',
        'switch_to'       => 'English',
        'switch_url'      => '?lang=en',

        'guard_done_title'   => 'Opsætningen er allerede gennemført',
        'guard_done_body'    => 'admin_tokens-tabellen indeholder allerede %d token(s). Guiden afviser at køre igen for at undgå at oprette ekstra credentials.',
        'guard_done_action'  => 'Slet denne fil (server/public/setup.php) — den er ikke længere nødvendig og udgør angrebs-flade.',

        'env_missing'        => 'Ingen .env-fil endnu. Guiden opretter en.',
        'env_unwritable'     => 'Kan ikke skrive til .env på %s. Sørg for at mappen er skrivbar for webserver-brugeren (chmod 700, ejet af din bruger).',
        'bootstrap_missing'  => 'Kunne ikke loade bootstrap.php (kiggede på %s). Server-koden er ikke på den forventede placering.',

        'step1_heading'      => 'Trin 1 — Database-forbindelse',
        'step1_intro'        => 'Indtast PostgreSQL-forbindelsesinfo. Guiden tester forbindelsen før den skriver til server/.env. Lad LOG_LEVEL stå på INFO medmindre du eksplicit vil have DEBUG.',
        'step1_dsn'          => 'PostgreSQL DSN',
        'step1_dsn_hint'     => 'Eksempel: pgsql:host=localhost;port=5432;dbname=keepass_deltasync',
        'step1_user'         => 'Database-bruger',
        'step1_password'     => 'Database-password',
        'step1_log'          => 'Log-niveau',
        'step1_audit'        => 'Audit-retention (dage)',
        'step1_ttl'          => 'Enrollment-token TTL (timer)',
        'step1_base'         => 'App-base-path (valgfri, fx /sync hvis under en sub-sti)',
        'step1_submit'       => 'Test forbindelse og gem .env',

        'step1_ok'           => 'Forbindelse OK. server/.env gemt.',
        'step1_fail'         => 'Forbindelse fejlede: %s',
        'step1_write_fail'   => 'Kunne ikke skrive .env: %s',

        'step2_heading'      => 'Trin 2 — Kør schema-migrationer',
        'step2_intro'        => 'Følgende SQL-filer køres mod databasen i rækkefølge. Hver er idempotent i den forstand at de fejler højlydt hvis deres tabeller allerede findes — så genkørsel er sikker; den stopper bare ved første eksisterende tabel.',
        'step2_no_files'     => 'Ingen schema/*.sql-filer fundet på %s.',
        'step2_run'          => 'Kør alle migrationer',
        'step2_ran'          => 'Kørte %d migrations-fil(er).',
        'step2_partial'      => 'Migration %s fejlede: %s. Tidligere filer er muligvis allerede kørt — tjek DB-status.',

        'step3_heading'      => 'Trin 3 — Første admin-token',
        'step3_intro'        => 'Generér det første admin-token. Det vises ÉN gang — kopiér det før du navigerer væk. Du bruger det med admin-CLI\'en: `keepass-deltasync admin user-create ...`.',
        'step3_run'          => 'Generér admin-token',
        'step3_token_label'  => 'Dit admin-token (gem det NU):',
        'step3_usage'        => 'Brug i HTTP-requests som: Authorization: Bearer &lt;token&gt;',
        'step3_env_hint'     => 'Eller sæt det som env-variabel til klient-CLI\'en:',

        'done_heading'       => 'Opsætning gennemført',
        'done_body'          => 'Serveren er klar. Gør så følgende:',
        'done_step_delete'   => 'Slet <code>server/public/setup.php</code> fra web-roden. Guiden afviser genkørsel, men filen er unødvendig angrebs-flade.',
        'done_step_user'     => 'Opret din første bruger med admin-CLI\'en:',
        'done_step_share'    => 'Send det udleverede enrollment-token til brugeren via en sikker kanal.',

        'btn_continue'       => 'Fortsæt',
    ],
];

$lang = $_GET['lang'] ?? $_POST['lang'] ?? 'en';
if (!isset($STRINGS[$lang])) {
    $lang = 'en';
}
$T = $STRINGS[$lang];

// ------------------------------------------------------------------
// Paths + helpers
// ------------------------------------------------------------------

$rootDir   = dirname(__DIR__);            // /server
$envPath   = $rootDir . '/.env';
$schemaDir = $rootDir . '/schema';
$bootstrap = $rootDir . '/bootstrap.php';

function h(string $s): string { return htmlspecialchars($s, ENT_QUOTES, 'UTF-8'); }

function escapeEnvValue(string $v): string {
    // Wrap in double quotes if it contains spaces or special chars.
    if ($v === '' || preg_match('/^[A-Za-z0-9_./:;=\-+]+$/', $v)) {
        return $v;
    }
    return '"' . str_replace(['\\', '"'], ['\\\\', '\\"'], $v) . '"';
}

/**
 * Generates a 32-byte URL-safe base64 token (matches TokenHasher::generate).
 */
function genToken(): string {
    return rtrim(strtr(base64_encode(random_bytes(32)), '+/', '-_'), '=');
}

function hashToken(string $token): string {
    return hash('sha256', $token);
}

// ------------------------------------------------------------------
// State detection
// ------------------------------------------------------------------

if (!is_file($bootstrap)) {
    // Cannot proceed without bootstrap.php — server/ code isn't here.
    http_response_code(500);
    header('Content-Type: text/plain; charset=utf-8');
    echo sprintf($T['bootstrap_missing'], $bootstrap) . "\n";
    exit;
}

// Try loading .env (if present). We do NOT use Config::loadFromEnv here
// because it throws on missing required vars and we want graceful fallback.
$envVars = [];
if (is_file($envPath)) {
    foreach (file($envPath, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES) as $line) {
        $line = trim($line);
        if ($line === '' || str_starts_with($line, '#')) { continue; }
        [$k, $v] = array_pad(explode('=', $line, 2), 2, '');
        $envVars[trim($k)] = trim(trim($v), '"');
    }
}

$hasEnv = isset($envVars['DATABASE_URL'], $envVars['DATABASE_USER'])
       && $envVars['DATABASE_URL'] !== ''
       && !str_contains($envVars['DATABASE_URL'], 'ChangeMe')
       && !str_contains($envVars['DATABASE_PASSWORD'] ?? '', 'ChangeMe');

// Build a PDO if env is present, else null.
$pdo = null;
$pdoErr = null;
if ($hasEnv) {
    try {
        $pdo = new PDO(
            $envVars['DATABASE_URL'],
            $envVars['DATABASE_USER'],
            $envVars['DATABASE_PASSWORD'] ?? '',
            [
                PDO::ATTR_ERRMODE            => PDO::ERRMODE_EXCEPTION,
                PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
                PDO::ATTR_EMULATE_PREPARES   => false,
            ],
        );
    } catch (\Throwable $e) {
        $pdoErr = $e->getMessage();
    }
}

// Check if admin_tokens already has entries.
$adminTokenCount = 0;
$adminTokensTableExists = false;
if ($pdo !== null) {
    try {
        $adminTokenCount = (int) $pdo->query('SELECT COUNT(*) FROM admin_tokens')->fetchColumn();
        $adminTokensTableExists = true;
    } catch (\Throwable $e) {
        // Table doesn't exist yet, fall through to step 2.
    }
}

// Determine current step:
//   0 = guard: already done
//   1 = collect DB credentials
//   2 = run migrations
//   3 = generate admin token
//   4 = done
$step = 1;
if ($hasEnv && $pdo !== null) {
    $step = $adminTokensTableExists ? 3 : 2;
}
if ($adminTokensTableExists && $adminTokenCount > 0) {
    $step = 0;
}

// ------------------------------------------------------------------
// POST handlers (mutate state)
// ------------------------------------------------------------------

$flash = null;          // success message
$flashError = null;     // error message
$generatedToken = null;

if ($_SERVER['REQUEST_METHOD'] === 'POST' && $step !== 0) {
    $action = $_POST['action'] ?? '';

    if ($action === 'save_env') {
        $newDsn   = trim($_POST['dsn']      ?? '');
        $newUser  = trim($_POST['db_user']  ?? '');
        $newPass  = (string) ($_POST['db_password'] ?? '');
        $newLog   = trim($_POST['log_level'] ?? 'INFO');
        $newAudit = trim($_POST['audit']    ?? '30');
        $newTtl   = trim($_POST['ttl']      ?? '24');
        $newBase  = trim($_POST['base_path'] ?? '');

        // Test connection first.
        try {
            $testPdo = new PDO($newDsn, $newUser, $newPass, [
                PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
            ]);
            $testPdo->query('SELECT 1');

            // Write .env atomically.
            $lines = [
                '# Generated by setup.php on ' . date('Y-m-d H:i:s'),
                'DATABASE_URL=' . escapeEnvValue($newDsn),
                'DATABASE_USER=' . escapeEnvValue($newUser),
                'DATABASE_PASSWORD=' . escapeEnvValue($newPass),
                'LOG_LEVEL=' . escapeEnvValue($newLog),
                'AUDIT_RETENTION_DAYS=' . escapeEnvValue($newAudit),
                'ENROLLMENT_TOKEN_TTL_HOURS=' . escapeEnvValue($newTtl),
            ];
            if ($newBase !== '') {
                $lines[] = 'APP_BASE_PATH=' . escapeEnvValue($newBase);
            }
            $content = implode("\n", $lines) . "\n";

            $tmpPath = $envPath . '.tmp';
            if (file_put_contents($tmpPath, $content, LOCK_EX) === false) {
                throw new \RuntimeException('write failed');
            }
            @chmod($tmpPath, 0600);
            if (!rename($tmpPath, $envPath)) {
                @unlink($tmpPath);
                throw new \RuntimeException('rename failed');
            }

            $flash = $T['step1_ok'];

            // Reload state — re-read .env, re-open PDO so the next render is consistent.
            $envVars['DATABASE_URL']      = $newDsn;
            $envVars['DATABASE_USER']     = $newUser;
            $envVars['DATABASE_PASSWORD'] = $newPass;
            $pdo = $testPdo;
            try {
                $adminTokenCount = (int) $pdo->query('SELECT COUNT(*) FROM admin_tokens')->fetchColumn();
                $adminTokensTableExists = true;
            } catch (\Throwable $e) {
                $adminTokensTableExists = false;
            }
            $step = $adminTokensTableExists ? 3 : 2;
        } catch (\PDOException $e) {
            $flashError = sprintf($T['step1_fail'], $e->getMessage());
        } catch (\Throwable $e) {
            $flashError = sprintf($T['step1_write_fail'], $e->getMessage());
        }
    } elseif ($action === 'run_migrations' && $pdo !== null) {
        $files = glob($schemaDir . '/*.sql') ?: [];
        sort($files);
        $ran = 0;
        try {
            foreach ($files as $file) {
                $sql = file_get_contents($file);
                if ($sql === false) { continue; }
                $pdo->exec($sql);
                $ran++;
            }
            $flash = sprintf($T['step2_ran'], $ran);
            // Refresh admin_tokens state.
            try {
                $adminTokenCount = (int) $pdo->query('SELECT COUNT(*) FROM admin_tokens')->fetchColumn();
                $adminTokensTableExists = true;
            } catch (\Throwable $e) {
                $adminTokensTableExists = false;
            }
            $step = $adminTokensTableExists ? 3 : 2;
        } catch (\Throwable $e) {
            $flashError = sprintf($T['step2_partial'], basename($file ?? ''), $e->getMessage());
        }
    } elseif ($action === 'gen_token' && $pdo !== null && $adminTokensTableExists) {
        try {
            $generatedToken = genToken();
            $stmt = $pdo->prepare('INSERT INTO admin_tokens (token_hash) VALUES (:h)');
            $stmt->execute(['h' => hashToken($generatedToken)]);
            $adminTokenCount = 1;
            $step = 4;  // Done page
        } catch (\Throwable $e) {
            $flashError = $e->getMessage();
        }
    }
}

?>
<!DOCTYPE html>
<html lang="<?= h($lang) ?>">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title><?= h($T['page_title']) ?></title>
<style>
:root {
    --bg: #ffffff;
    --bg-elev: #f8fafc;
    --bg-code: #f1f5f9;
    --text: #0f172a;
    --muted: #475569;
    --border: #e2e8f0;
    --accent: #1e40af;
    --accent-bright: #38bdf8;
    --gold: #f59e0b;
    --danger: #b91c1c;
    --success: #15803d;
}
@media (prefers-color-scheme: dark) {
    :root {
        --bg: #0a0f1c;
        --bg-elev: #131b2e;
        --bg-code: #1a2540;
        --text: #e2e8f0;
        --muted: #94a3b8;
        --border: #1e293b;
        --accent: #60a5fa;
        --accent-bright: #38bdf8;
        --gold: #fde047;
        --danger: #f87171;
        --success: #4ade80;
    }
}
* { box-sizing: border-box; }
body {
    margin: 0;
    background: var(--bg);
    color: var(--text);
    font-family: system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
    font-size: 16px;
    line-height: 1.6;
}
.wrap {
    max-width: 720px;
    margin: 0 auto;
    padding: 2rem 1.5rem;
}
header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 1rem;
    margin-bottom: 2rem;
}
h1 {
    font-size: 1.8rem;
    margin: 0 0 0.5rem;
    line-height: 1.2;
}
.subhead {
    color: var(--muted);
    margin: 0;
    font-size: 0.95rem;
}
.lang a {
    color: var(--muted);
    text-decoration: none;
    font-size: 0.85rem;
    border: 1px solid var(--border);
    padding: 4px 10px;
    border-radius: 4px;
}
.lang a:hover { color: var(--text); background: var(--bg-elev); }
section.step {
    background: var(--bg-elev);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 1.5rem;
    margin-bottom: 1.5rem;
}
h2 { font-size: 1.25rem; margin: 0 0 0.5rem; }
form .row { margin-bottom: 1rem; }
label {
    display: block;
    font-weight: 600;
    margin-bottom: 0.3rem;
    font-size: 0.9rem;
}
label .hint {
    font-weight: 400;
    color: var(--muted);
    font-size: 0.8rem;
    margin-left: 0.5rem;
}
input[type="text"], input[type="password"], select {
    width: 100%;
    padding: 0.55rem 0.7rem;
    font-size: 0.95rem;
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg);
    color: var(--text);
    font-family: ui-monospace, "Cascadia Code", Menlo, Consolas, monospace;
}
.btn {
    background: var(--accent-bright);
    color: #0f172a;
    border: none;
    padding: 0.6rem 1.2rem;
    font-size: 0.95rem;
    font-weight: 600;
    border-radius: 6px;
    cursor: pointer;
    transition: transform 0.1s;
}
.btn:hover { transform: translateY(-1px); }
.flash {
    padding: 0.8rem 1rem;
    border-radius: 6px;
    margin-bottom: 1.5rem;
    border-left: 4px solid;
}
.flash.success { background: var(--bg-elev); border-color: var(--success); color: var(--success); }
.flash.error { background: var(--bg-elev); border-color: var(--danger); color: var(--danger); }
.token {
    background: var(--bg-code);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 1rem;
    font-family: ui-monospace, monospace;
    font-size: 1.1rem;
    word-break: break-all;
    user-select: all;
    margin: 1rem 0;
    color: var(--gold);
}
pre, code {
    background: var(--bg-code);
    font-family: ui-monospace, monospace;
    font-size: 0.85rem;
}
pre {
    padding: 0.8rem 1rem;
    border-radius: 6px;
    border: 1px solid var(--border);
    overflow-x: auto;
}
code {
    padding: 1px 6px;
    border-radius: 3px;
}
ul.files { list-style: none; padding-left: 0; }
ul.files li {
    padding: 0.3rem 0;
    font-family: ui-monospace, monospace;
    font-size: 0.85rem;
    color: var(--muted);
}
ol.done {
    padding-left: 1.25rem;
}
ol.done li { margin-bottom: 1rem; }
.warn {
    color: var(--gold);
    font-weight: 600;
}
</style>
</head>
<body>
<div class="wrap">

<header>
    <div>
        <h1><?= h($T['heading']) ?></h1>
        <p class="subhead"><?= h($T['subheading']) ?></p>
    </div>
    <div class="lang">
        <a href="<?= h($T['switch_url']) ?>"><?= h($T['switch_to']) ?></a>
    </div>
</header>

<?php if ($flash !== null): ?>
    <div class="flash success"><?= h($flash) ?></div>
<?php endif; ?>
<?php if ($flashError !== null): ?>
    <div class="flash error"><?= h($flashError) ?></div>
<?php endif; ?>
<?php if ($pdoErr !== null && $step !== 0 && $step !== 4): ?>
    <div class="flash error"><?= h(sprintf($T['step1_fail'], $pdoErr)) ?></div>
<?php endif; ?>

<?php if ($step === 0): ?>
    <section class="step">
        <h2><?= h($T['guard_done_title']) ?></h2>
        <p><?= h(sprintf($T['guard_done_body'], $adminTokenCount)) ?></p>
        <p class="warn"><?= h($T['guard_done_action']) ?></p>
    </section>

<?php elseif ($step === 1): ?>
    <section class="step">
        <h2><?= h($T['step1_heading']) ?></h2>
        <p><?= h($T['step1_intro']) ?></p>
        <?php if (!is_writable(dirname($envPath)) && !is_writable($envPath)): ?>
            <div class="flash error"><?= h(sprintf($T['env_unwritable'], dirname($envPath))) ?></div>
        <?php endif; ?>

        <form method="post">
            <input type="hidden" name="action" value="save_env">
            <input type="hidden" name="lang" value="<?= h($lang) ?>">

            <div class="row">
                <label><?= h($T['step1_dsn']) ?><span class="hint"><?= h($T['step1_dsn_hint']) ?></span></label>
                <input type="text" name="dsn" required value="<?= h($envVars['DATABASE_URL'] ?? 'pgsql:host=localhost;port=5432;dbname=keepass_deltasync') ?>">
            </div>
            <div class="row">
                <label><?= h($T['step1_user']) ?></label>
                <input type="text" name="db_user" required value="<?= h($envVars['DATABASE_USER'] ?? '') ?>">
            </div>
            <div class="row">
                <label><?= h($T['step1_password']) ?></label>
                <input type="password" name="db_password" required>
            </div>
            <div class="row">
                <label><?= h($T['step1_log']) ?></label>
                <select name="log_level">
                    <option value="INFO" <?= ($envVars['LOG_LEVEL'] ?? '') === 'INFO' ? 'selected' : '' ?>>INFO</option>
                    <option value="DEBUG" <?= ($envVars['LOG_LEVEL'] ?? '') === 'DEBUG' ? 'selected' : '' ?>>DEBUG</option>
                </select>
            </div>
            <div class="row">
                <label><?= h($T['step1_audit']) ?></label>
                <input type="text" name="audit" value="<?= h($envVars['AUDIT_RETENTION_DAYS'] ?? '30') ?>">
            </div>
            <div class="row">
                <label><?= h($T['step1_ttl']) ?></label>
                <input type="text" name="ttl" value="<?= h($envVars['ENROLLMENT_TOKEN_TTL_HOURS'] ?? '24') ?>">
            </div>
            <div class="row">
                <label><?= h($T['step1_base']) ?></label>
                <input type="text" name="base_path" value="<?= h($envVars['APP_BASE_PATH'] ?? '') ?>" placeholder="/sync">
            </div>

            <button class="btn" type="submit"><?= h($T['step1_submit']) ?></button>
        </form>
    </section>

<?php elseif ($step === 2): ?>
    <section class="step">
        <h2><?= h($T['step2_heading']) ?></h2>
        <p><?= h($T['step2_intro']) ?></p>

        <?php $files = glob($schemaDir . '/*.sql') ?: []; sort($files); ?>
        <?php if (empty($files)): ?>
            <div class="flash error"><?= h(sprintf($T['step2_no_files'], $schemaDir)) ?></div>
        <?php else: ?>
            <ul class="files">
                <?php foreach ($files as $f): ?>
                    <li>📄 <?= h(basename($f)) ?></li>
                <?php endforeach; ?>
            </ul>
            <form method="post">
                <input type="hidden" name="action" value="run_migrations">
                <input type="hidden" name="lang" value="<?= h($lang) ?>">
                <button class="btn" type="submit"><?= h($T['step2_run']) ?></button>
            </form>
        <?php endif; ?>
    </section>

<?php elseif ($step === 3): ?>
    <section class="step">
        <h2><?= h($T['step3_heading']) ?></h2>
        <p><?= h($T['step3_intro']) ?></p>
        <form method="post">
            <input type="hidden" name="action" value="gen_token">
            <input type="hidden" name="lang" value="<?= h($lang) ?>">
            <button class="btn" type="submit"><?= h($T['step3_run']) ?></button>
        </form>
    </section>

<?php elseif ($step === 4 && $generatedToken !== null): ?>
    <section class="step">
        <h2><?= h($T['done_heading']) ?></h2>
        <p><strong><?= h($T['step3_token_label']) ?></strong></p>
        <div class="token"><?= h($generatedToken) ?></div>
        <p><?= $T['step3_usage'] ?></p>
        <p><?= h($T['step3_env_hint']) ?></p>
<pre><code># PowerShell
$env:KEEPASS_DELTASYNC_ADMIN_TOKEN = "<?= h($generatedToken) ?>"
# bash
export KEEPASS_DELTASYNC_ADMIN_TOKEN="<?= h($generatedToken) ?>"</code></pre>

        <h2 style="margin-top:2rem;"><?= h($T['done_body']) ?></h2>
        <ol class="done">
            <li><?= $T['done_step_delete'] ?></li>
            <li>
                <?= h($T['done_step_user']) ?>
<pre><code>keepass-deltasync admin user-create alice --display-name "Alice"</code></pre>
            </li>
            <li><?= h($T['done_step_share']) ?></li>
        </ol>
    </section>
<?php endif; ?>

</div>
</body>
</html>
