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
// UI is in English by default; ?lang= switches to any language in $LANGS
// (da, de, fr, es). Unknown values fall back to English.

declare(strict_types=1);

// ------------------------------------------------------------------
// Localization
// ------------------------------------------------------------------

$STRINGS = [
    'en' => [
        'page_title'      => 'DeltaSync Setup',
        'heading'         => 'DeltaSync — First-time setup',
        'subheading'      => 'A short wizard that prepares the server. Run it once, save the admin token, then delete this file.',

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
    'de' => [
        'page_title'      => 'DeltaSync-Einrichtung',
        'heading'         => 'DeltaSync — Ersteinrichtung',
        'subheading'      => 'Ein kurzer Assistent, der den Server vorbereitet. Einmal ausführen, das Admin-Token speichern, dann diese Datei löschen.',

        'guard_done_title'   => 'Einrichtung bereits abgeschlossen',
        'guard_done_body'    => 'Die Tabelle admin_tokens enthält bereits %d Einträge. Dieser Assistent führt sich nicht erneut aus, um nicht versehentlich zusätzliche Zugangsdaten anzulegen.',
        'guard_done_action'  => 'Löschen Sie diese Datei (server/public/setup.php) — sie wird nicht mehr benötigt und vergrößert die Angriffsfläche.',

        'env_missing'        => 'Noch keine .env-Datei gefunden. Der Assistent legt eine an.',
        'env_unwritable'     => 'Kann .env unter %s nicht schreiben. Stellen Sie sicher, dass das Verzeichnis für den Webserver-Benutzer beschreibbar ist (chmod 700, Ihrem Benutzer gehörend).',
        'bootstrap_missing'  => 'bootstrap.php konnte nicht geladen werden (gesucht unter %s). Der Server-Code liegt nicht an der erwarteten Stelle.',

        'step1_heading'      => 'Schritt 1 — Datenbankverbindung',
        'step1_intro'        => 'Geben Sie die PostgreSQL-Verbindungsdaten ein. Der Assistent testet die Verbindung, bevor er sie nach server/.env schreibt. Lassen Sie LOG_LEVEL auf INFO, sofern Sie nicht bewusst DEBUG möchten.',
        'step1_dsn'          => 'PostgreSQL-DSN',
        'step1_dsn_hint'     => 'Beispiel: pgsql:host=localhost;port=5432;dbname=keepass_deltasync',
        'step1_user'         => 'Datenbankbenutzer',
        'step1_password'     => 'Datenbankpasswort',
        'step1_log'          => 'Protokollstufe',
        'step1_audit'        => 'Audit-Aufbewahrung (Tage)',
        'step1_ttl'          => 'Gültigkeit des Enrollment-Tokens (Stunden)',
        'step1_base'         => 'Basispfad der App (optional, z. B. /sync bei Betrieb unter einem Unterpfad)',
        'step1_submit'       => 'Verbindung testen und .env speichern',

        'step1_ok'           => 'Verbindung erfolgreich. server/.env gespeichert.',
        'step1_fail'         => 'Verbindung fehlgeschlagen: %s',
        'step1_write_fail'   => '.env konnte nicht geschrieben werden: %s',

        'step2_heading'      => 'Schritt 2 — Schema-Migrationen ausführen',
        'step2_intro'        => 'Die folgenden SQL-Dateien werden der Reihe nach gegen die Datenbank ausgeführt. Jede ist in dem Sinne idempotent, dass sie lautstark fehlschlägt, wenn ihre Tabellen bereits existieren — ein erneuter Lauf ist also sicher; er stoppt einfach bei der ersten vorhandenen Tabelle.',
        'step2_no_files'     => 'Keine schema/*.sql-Dateien unter %s gefunden.',
        'step2_run'          => 'Alle Migrationen ausführen',
        'step2_ran'          => '%d Migrationsdatei(en) ausgeführt.',
        'step2_partial'      => 'Migration %s fehlgeschlagen: %s. Frühere Dateien wurden möglicherweise schon angewendet — prüfen Sie den Zustand Ihrer Datenbank.',

        'step3_heading'      => 'Schritt 3 — Erstes Admin-Token',
        'step3_intro'        => 'Erzeugen Sie das erste Admin-Token. Es wird nur EINMAL angezeigt — kopieren Sie es, bevor Sie die Seite verlassen. Sie verwenden es mit der Admin-CLI: `keepass-deltasync admin user-create ...`.',
        'step3_run'          => 'Admin-Token erzeugen',
        'step3_token_label'  => 'Ihr Admin-Token (jetzt SOFORT speichern):',
        'step3_usage'        => 'In HTTP-Anfragen verwenden als: Authorization: Bearer &lt;token&gt;',
        'step3_env_hint'     => 'Oder als Umgebungsvariable für die Client-CLI setzen:',

        'done_heading'       => 'Einrichtung abgeschlossen',
        'done_body'          => 'Der Server ist bereit. Tun Sie jetzt Folgendes:',
        'done_step_delete'   => 'Löschen Sie <code>server/public/setup.php</code> aus dem Web-Root. Der Assistent verweigert einen erneuten Lauf, aber die Datei ist unnötige Angriffsfläche.',
        'done_step_user'     => 'Legen Sie Ihren ersten Benutzer mit der Admin-CLI an:',
        'done_step_share'    => 'Senden Sie das erhaltene Enrollment-Token über einen sicheren Kanal an diesen Benutzer.',

        'btn_continue'       => 'Weiter',
    ],
    'fr' => [
        'page_title'      => 'Installation de DeltaSync',
        'heading'         => 'DeltaSync — Première installation',
        'subheading'      => 'Un bref assistant qui prépare le serveur. Exécutez-le une fois, enregistrez le jeton d\'administration, puis supprimez ce fichier.',

        'guard_done_title'   => 'Installation déjà terminée',
        'guard_done_body'    => 'La table admin_tokens contient déjà %d entrées. Cet assistant refuse de s\'exécuter à nouveau afin de ne pas créer d\'identifiants supplémentaires par inadvertance.',
        'guard_done_action'  => 'Supprimez ce fichier (server/public/setup.php) — il n\'est plus nécessaire et augmente la surface d\'attaque.',

        'env_missing'        => 'Aucun fichier .env trouvé pour l\'instant. L\'assistant va en créer un.',
        'env_unwritable'     => 'Impossible d\'écrire dans .env à %s. Assurez-vous que le répertoire est accessible en écriture par l\'utilisateur du serveur web (chmod 700, appartenant à votre utilisateur).',
        'bootstrap_missing'  => 'Impossible de charger bootstrap.php (cherché à %s). Le code du serveur n\'est pas à l\'emplacement attendu.',

        'step1_heading'      => 'Étape 1 — Connexion à la base de données',
        'step1_intro'        => 'Saisissez les informations de connexion PostgreSQL. L\'assistant teste la connexion avant de les écrire dans server/.env. Laissez LOG_LEVEL sur INFO sauf si vous voulez délibérément DEBUG.',
        'step1_dsn'          => 'DSN PostgreSQL',
        'step1_dsn_hint'     => 'Exemple : pgsql:host=localhost;port=5432;dbname=keepass_deltasync',
        'step1_user'         => 'Utilisateur de la base de données',
        'step1_password'     => 'Mot de passe de la base de données',
        'step1_log'          => 'Niveau de journalisation',
        'step1_audit'        => 'Conservation de l\'audit (jours)',
        'step1_ttl'          => 'Durée de validité du jeton d\'inscription (heures)',
        'step1_base'         => 'Chemin de base de l\'application (facultatif, par ex. /sync si servie sous un sous-chemin)',
        'step1_submit'       => 'Tester la connexion et enregistrer .env',

        'step1_ok'           => 'Connexion réussie. server/.env enregistré.',
        'step1_fail'         => 'Échec de la connexion : %s',
        'step1_write_fail'   => 'Impossible d\'écrire .env : %s',

        'step2_heading'      => 'Étape 2 — Exécuter les migrations de schéma',
        'step2_intro'        => 'Les fichiers SQL suivants seront exécutés dans l\'ordre sur la base de données. Chacun est idempotent au sens où il échoue bruyamment si ses tables existent déjà — relancer est donc sans risque ; le processus s\'arrête simplement à la première table existante.',
        'step2_no_files'     => 'Aucun fichier schema/*.sql trouvé à %s.',
        'step2_run'          => 'Exécuter toutes les migrations',
        'step2_ran'          => '%d fichier(s) de migration exécuté(s).',
        'step2_partial'      => 'Échec de la migration %s : %s. Des fichiers précédents ont peut-être été appliqués — vérifiez l\'état de votre base de données.',

        'step3_heading'      => 'Étape 3 — Premier jeton d\'administration',
        'step3_intro'        => 'Générez le jeton d\'administration initial. Il n\'est affiché qu\'UNE SEULE fois — copiez-le avant de quitter la page. Vous l\'utilisez avec la CLI d\'administration : `keepass-deltasync admin user-create ...`.',
        'step3_run'          => 'Générer le jeton d\'administration',
        'step3_token_label'  => 'Votre jeton d\'administration (enregistrez-le MAINTENANT) :',
        'step3_usage'        => 'À utiliser dans les requêtes HTTP ainsi : Authorization: Bearer &lt;token&gt;',
        'step3_env_hint'     => 'Ou définissez-le comme variable d\'environnement pour la CLI cliente :',

        'done_heading'       => 'Installation terminée',
        'done_body'          => 'Le serveur est prêt. Faites maintenant ceci :',
        'done_step_delete'   => 'Supprimez <code>server/public/setup.php</code> de la racine web. L\'assistant refusera de s\'exécuter à nouveau, mais le fichier reste une surface d\'attaque inutile.',
        'done_step_user'     => 'Créez votre premier utilisateur avec la CLI d\'administration :',
        'done_step_share'    => 'Envoyez le jeton d\'inscription obtenu à cet utilisateur par un canal sécurisé.',

        'btn_continue'       => 'Continuer',
    ],
    'es' => [
        'page_title'      => 'Configuración de DeltaSync',
        'heading'         => 'DeltaSync — Configuración inicial',
        'subheading'      => 'Un asistente breve que prepara el servidor. Ejecútelo una vez, guarde el token de administración y luego elimine este archivo.',

        'guard_done_title'   => 'La configuración ya está completa',
        'guard_done_body'    => 'La tabla admin_tokens ya contiene %d entradas. Este asistente se niega a ejecutarse de nuevo para no crear credenciales adicionales por accidente.',
        'guard_done_action'  => 'Elimine este archivo (server/public/setup.php) — ya no es necesario y amplía la superficie de ataque.',

        'env_missing'        => 'Aún no se ha encontrado ningún archivo .env. El asistente creará uno.',
        'env_unwritable'     => 'No se puede escribir en .env en %s. Asegúrese de que el directorio sea escribible por el usuario del servidor web (chmod 700, propiedad de su usuario).',
        'bootstrap_missing'  => 'No se pudo cargar bootstrap.php (se buscó en %s). El código del servidor no está en la ubicación esperada.',

        'step1_heading'      => 'Paso 1 — Conexión a la base de datos',
        'step1_intro'        => 'Introduzca los datos de conexión de PostgreSQL. El asistente probará la conexión antes de escribirlos en server/.env. Deje LOG_LEVEL en INFO salvo que quiera DEBUG a propósito.',
        'step1_dsn'          => 'DSN de PostgreSQL',
        'step1_dsn_hint'     => 'Ejemplo: pgsql:host=localhost;port=5432;dbname=keepass_deltasync',
        'step1_user'         => 'Usuario de la base de datos',
        'step1_password'     => 'Contraseña de la base de datos',
        'step1_log'          => 'Nivel de registro',
        'step1_audit'        => 'Retención de auditoría (días)',
        'step1_ttl'          => 'Validez del token de inscripción (horas)',
        'step1_base'         => 'Ruta base de la aplicación (opcional, p. ej. /sync si se sirve bajo una subruta)',
        'step1_submit'       => 'Probar la conexión y guardar .env',

        'step1_ok'           => 'Conexión correcta. server/.env guardado.',
        'step1_fail'         => 'La conexión falló: %s',
        'step1_write_fail'   => 'No se pudo escribir .env: %s',

        'step2_heading'      => 'Paso 2 — Ejecutar las migraciones del esquema',
        'step2_intro'        => 'Los siguientes archivos SQL se ejecutarán en orden sobre la base de datos. Cada uno es idempotente en el sentido de que falla ruidosamente si sus tablas ya existen, así que volver a ejecutarlos es seguro; simplemente se detiene en la primera tabla existente.',
        'step2_no_files'     => 'No se encontraron archivos schema/*.sql en %s.',
        'step2_run'          => 'Ejecutar todas las migraciones',
        'step2_ran'          => 'Se ejecutaron %d archivo(s) de migración.',
        'step2_partial'      => 'La migración %s falló: %s. Puede que ya se hayan aplicado archivos anteriores — compruebe el estado de su base de datos.',

        'step3_heading'      => 'Paso 3 — Primer token de administración',
        'step3_intro'        => 'Genere el token de administración inicial. Se muestra UNA SOLA vez — cópielo antes de salir de la página. Se usa con la CLI de administración: `keepass-deltasync admin user-create ...`.',
        'step3_run'          => 'Generar token de administración',
        'step3_token_label'  => 'Su token de administración (guárdelo AHORA):',
        'step3_usage'        => 'Úselo en las peticiones HTTP así: Authorization: Bearer &lt;token&gt;',
        'step3_env_hint'     => 'O defínalo como variable de entorno para la CLI del cliente:',

        'done_heading'       => 'Configuración completada',
        'done_body'          => 'El servidor está listo. Ahora haga lo siguiente:',
        'done_step_delete'   => 'Elimine <code>server/public/setup.php</code> de la raíz web. El asistente se negará a ejecutarse de nuevo, pero el archivo es superficie de ataque innecesaria.',
        'done_step_user'     => 'Cree su primer usuario con la CLI de administración:',
        'done_step_share'    => 'Envíe el token de inscripción resultante a ese usuario por un canal seguro.',

        'btn_continue'       => 'Continuar',
    ],
];

// The wizard's languages, in the order they appear in the header. The key is
// both the ?lang= value and the <html lang> attribute; the label is each
// language's own name, since someone who needs the switcher cannot be assumed
// to read the language currently on screen.
$LANGS = [
    'en' => 'English',
    'da' => 'Dansk',
    'de' => 'Deutsch',
    'fr' => 'Français',
    'es' => 'Español',
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
    flex-wrap: wrap;
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
/* Fem sprognavne fylder ca. 410px, og indholdsspalten er 672px bred (720 minus
   padding). Raekken kan derfor aldrig staa ved siden af overskriften, som den
   gamle enkelt-knap kunne — saa den faar sin egen linje under den i stedet for
   at ombryde til tre rodede rader. Den ombryder foerst internt under ca. 420px
   skaermbredde. */
header > div:first-child { flex: 1 1 100%; }
.lang {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
}
.lang a, .lang .current {
    font-size: 0.85rem;
    border: 1px solid var(--border);
    padding: 4px 10px;
    border-radius: 4px;
}
.lang .current {
    color: var(--text);
    border-color: var(--accent);
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
<?php foreach ($LANGS as $code => $label): ?>
<?php if ($code === $lang): ?>
        <span class="current" aria-current="true"><?= h($label) ?></span>
<?php else: ?>
        <a href="?lang=<?= h($code) ?>" hreflang="<?= h($code) ?>"><?= h($label) ?></a>
<?php endif; ?>
<?php endforeach; ?>
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
