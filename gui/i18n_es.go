// SPDX-License-Identifier: GPL-3.0-or-later

package main

// Español. Traducido del inglés; el orden de los campos sigue a langEN en
// i18n.go para que ambos sean fáciles de comparar.

func init() {
	dicts[langES] = &dict{
		AppTitle: "KeePass Delta-Sync",

		OK: "Aceptar", Cancel: "Cancelar", Close: "Cerrar", Save: "Guardar",
		Back: "Atrás", Next: "Siguiente", Finish: "Finalizar", Browse: "Examinar…",
		Working: "Trabajando…", Done: "Hecho", Error: "Error",
		Copy: "Copiar", DBInfoTitle: "Información de la base de datos",

		CLINotFound:     "No se encontró el programa «keepass-deltasync».",
		CLILocate:       "Localizar keepass-deltasync",
		CLILocateHint:   "Indique el programa keepass-deltasync (la CLI). Normalmente se distribuye junto a esta aplicación.",
		CLIPathLabel:    "Ruta de la CLI",
		CLISelectBinary: "Seleccionar el programa keepass-deltasync",

		WizardWelcomeTitle: "Bienvenido",
		WizardWelcomeBody: "Este asistente le ayuda a empezar a sincronizar su base de datos de KeePass " +
			"entre sus dispositivos.\n\nNecesitará:\n  • La dirección del servidor que le dé su administrador\n  • Un token de inscripción (código de un solo uso)\n  • Su archivo .kdbx",
		WizardStart:      "Iniciar el asistente",
		WizardStepEnroll: "Paso 1 de 2 — Registrar el dispositivo",
		WizardStepDB:     "Paso 2 de 2 — Añadir base de datos",
		ServerURL:        "Dirección del servidor",
		ServerURLHint:    "p. ej. https://deltasync.ejemplo.es",
		EnrollToken:      "Token de inscripción",
		EnrollTokenHint:  "El código de un solo uso que le dé su administrador",
		DeviceName:       "Nombre del dispositivo (opcional)",
		DeviceNameHint:   "Se muestra en los listados de administración. Vacío = nombre del equipo.",
		EnrollButton:     "Registrar este dispositivo",
		EnrollOK:         "¡Dispositivo registrado!",
		WizardAddDB:      "Añada su primera base de datos",
		WizardAddDBBody:  "Registre un archivo .kdbx local para sincronizarlo. Siempre podrá añadir más más adelante.",
		DBName:           "Nombre",
		DBNameHint:       "Un nombre corto, p. ej. «personal» o «trabajo»",
		KdbxFile:         "Archivo de KeePass (.kdbx)",
		KdbxFileHint:     "Ruta de su base de datos local",
		CreateDBButton:   "Crear base de datos",
		SkipForNow:       "Omitir por ahora",
		WizardDone:       "¡Todo listo!",
		WizardDoneBody:   "Ya puede sincronizar. Use el botón «Sync» de una base de datos para enviar y recibir cambios.",

		WizardAdvanced:     "Avanzado (administrador)",
		WizardFirefox:      "Buscar desde Firefox — sin cuenta",
		FFIntro:            "Busque en sus entradas de KeePass desde Firefox y vaya directo al sitio correcto. No hace falta cuenta ni servidor, solo dos cosas, una vez: indicar su .kdbx al programa y registrarlo en Firefox. Rellenar las credenciales sigue siendo cosa de KeePassXC-Browser; la extensión nunca ve una contraseña.",
		FFAddLocal:         "Registrar base de datos…",
		FFAddLocalTitle:    "Registrar una base de datos para la búsqueda",
		FFAddLocalDone:     "La base de datos está registrada. A continuación: Registrar en Firefox y reiniciar Firefox.",
		FFSavePassword:     "Guardar la contraseña maestra en el llavero del sistema (para que la ventana no la pida)",
		FFPasswordMissing:  "Escriba la contraseña maestra, o desmarque la casilla; entonces la pedirá la extensión.",
		FFInstallHost:      "Registrar en Firefox",
		FFHostInstalled:    "Registrado en Firefox",
		FFRestartFirefox:   "Reinicie Firefox: es lo que hace que tome el registro.",
		FFPreview:          "Mostrar lo que se escribiría",
		FFPreviewTitle:     "Simulación (no se escribe nada)",
		FFUninstallHost:    "Eliminar el registro",
		FFUninstallConfirm: "¿Eliminar el registro de todas las variantes de Firefox de este equipo? Ni la base de datos ni el archivo se tocan.",
		FFGuide:            "Guía",
		FFProbe:            "Probar",
		FFProbeTitle:       "Lo que recibiría el navegador — %s",
		FFProbePwdHint:     "Opcional: se toma del llavero si está guardada ahí",
		FFProbeEmpty:       "El índice está vacío: no hay entradas con una URL utilizable.",
		FFCount:            "%d base(s) de datos consultable(s) desde Firefox",
		FFNone:             "Aún no hay bases de datos registradas; empiece por Registrar base de datos.",
		FFKindSynced:       "(sincronizada)",
		FFKindLocal:        "(solo local)",
		FFCLIPath:          "Programa:",
		HelpFirefox: "## Firefox\n\n" +
			"Búsqueda de texto libre en sus entradas desde la barra de direcciones de Firefox (`kp` + espacio) o desde la ventana emergente (**Alt+Mayús+K**), y una tecla abre el sitio de la entrada. La extensión recibe **solo** el título, las URL y la ruta del grupo; nunca contraseñas, nombres de usuario, notas ni archivos adjuntos.\n\n" +
			"La configuración son dos pasos, una vez por equipo:\n\n" +
			"- **Registrar base de datos**: indica un `.kdbx` al programa (`add-local`). Sin servidor, sin cuenta, no se sube nada. Marque la casilla y la contraseña maestra pasa al llavero del sistema, de modo que la ventana no la pida.\n" +
			"- **Registrar en Firefox**: escribe el manifiesto de mensajería nativa para que Firefox pueda iniciar el host (`install-browser-host`). **Reinicie Firefox después.**\n\n" +
			"Además: **Probar** muestra el índice exacto que recibiría el navegador (`browser-host --probe`), y **Mostrar lo que se escribiría** hace el mismo registro sin tocar nada (`--dry-run`).\n\n" +
			"El complemento se instala desde addons.mozilla.org; consulte la **Guía**.\n\n" +
			"**Órdenes de la CLI:**\n\n" +
			"- `keepass-deltasync add-local <name> <file.kdbx>`\n" +
			"- `keepass-deltasync install-browser-host`\n" +
			"- `keepass-deltasync browser-host --probe <name>`\n" +
			"- `keepass-deltasync uninstall-browser-host`",
		WizardStepAdvanced: "Registro avanzado — administrador",
		AdvancedIntro: "Si dispone de un token de administración, puede emitir un token de inscripción y registrar este PC en un solo paso: " +
			"no hace falta ningún token de antemano. Elija un usuario existente o cree uno nuevo.",
		AdvUserMode:     "Usuario",
		AdvExistingUser: "Usuario existente",
		AdvNewUser:      "Crear usuario nuevo",
		AdvEnrollButton: "Emitir token y registrar este PC",
		AdvIssuingToken: "Emitiendo el token de inscripción…",
		AdvEnrolling:    "Registrando este PC…",
		AdvNoTokenErr:   "No se pudo leer el token de inscripción en la respuesta del servidor. Consulte el registro para más detalles.",

		TabDatabases: "Bases de datos",
		TabDevices:   "Dispositivos",
		TabFirefox:   "Firefox",
		TabActivity:  "Actividad",
		TabLog:       "Registro",
		TabAdmin:     "Administración",
		TabSettings:  "Ajustes",
		ActivityHint: "Aquí aparece la salida de las llamadas a la CLI (sincronización, añadir/quitar, errores …).",
		Clear:        "Limpiar",

		LogHint:         "Registro de actividad del servidor (auditoría): el historial de todos los dispositivos, conservado hasta 30 días.",
		LogCount:        "%d entradas de registro",
		LogEmpty:        "(no hay entradas de registro en este periodo)",
		LogPeriodLabel:  "Periodo:",
		LogPeriod24h:    "Últimas 24 horas",
		LogPeriod7d:     "Últimos 7 días",
		LogPeriod30d:    "Últimos 30 días",
		LogPeriodAll:    "Todo",
		LogDetailsTitle: "Detalles del registro",
		LogColTime:      "Hora",
		LogColEvent:     "Evento",
		LogColLevel:     "Nivel",
		LogColIP:        "Dirección IP",
		LogOK:           "Correcto",
		LogFail:         "Fallido",

		ColEnrolled:     "Registrado",
		ColLastSeen:     "Visto por última vez",
		ThisDevice:      "● este dispositivo",
		DevCount:        "%d dispositivo(s) en la cuenta",
		DeviceInfoTitle: "Información del dispositivo",

		AddDevice:           "Añadir dispositivo",
		AddDeviceCreate:     "Generar token",
		RemoveDevice:        "Quitar dispositivo",
		Username:            "Nombre de usuario",
		AdminToken:          "Token de administración",
		EnrollTokenCreated:  "Token de inscripción para el nuevo dispositivo",
		SelectDeviceFirst:   "Seleccione primero un dispositivo en la lista.",
		CannotRemoveCurrent: "Desde aquí no puede quitar el dispositivo que está usando.",
		ConfirmRemoveDevice: "¿Quitar (revocar) el dispositivo %q? Su token dejará de ser válido.",
		StatusBox:           "Estado",
		NotEnrolled:         "Aún sin registrar.",
		Refresh:             "Actualizar",
		Sync:                "Sync",
		SyncSelected:        "Sincronizar lo seleccionado",
		SyncAll:             "Sincronizar todo",
		SelectFirst:         "Seleccione primero una base de datos en la lista.",
		AddDatabase:         "Añadir base de datos",
		ForgetDatabase:      "Olvidar base de datos",
		ConfirmForget:       "¿Olvidar el vínculo local con %q? El archivo .kdbx y la base de datos del servidor NO se tocan; solo se elimina el vínculo en este cliente.",
		MoreActions:         "Más acciones",
		PushNow:             "Enviar ahora (solo subida)",
		PullNow:             "Recibir ahora (solo descarga)",
		DeleteOnServer:      "Eliminar en el servidor",
		ConfirmDeleteServer: "¿Eliminar DEFINITIVAMENTE la base de datos %q en el servidor?\n\nEsto quita TODAS las entradas, versiones, comparticiones e historial, para todos los usuarios. Esta acción NO se puede deshacer. Su archivo .kdbx local queda intacto.",
		NoDatabases:         "Aún no hay bases de datos. Pulse «Añadir base de datos» para empezar.",
		DBCount:             "%d base(s) de datos conectada(s)",
		ColName:             "Nombre",
		ColStatus:           "Estado",
		ColID:               "ID",
		ColCreated:          "Creada",
		ColPath:             "Ruta local",
		BoundLocally:        "● lista",
		OnServerOnly:        "○ solo en el servidor",

		MembersTitle:       "Conectados a la base de datos",
		MembersOf:          "Conectados a %q",
		SelectToSeeMembers: "Seleccione arriba una base de datos para ver quién está conectado a ella.",
		MemberCount:        "%d miembro(s)",
		MembersNeedBound:   "La base de datos debe estar configurada localmente para mostrar los miembros.",
		MembersUnavailable: "No se pueden obtener los miembros (solo quien es propietario puede verlos):",
		NoMembers:          "Solo usted: aún no está compartida con nadie.",
		ColRole:            "Función",
		ColUser:            "Nombre de usuario",
		ColDisplay:         "Nombre visible",
		ColAdded:           "Añadido",

		ShareDatabase:     "Compartir base de datos",
		ShareTitle:        "Compartir %q con un usuario",
		ShareWith:         "Compartir con (nombre de usuario)",
		RemoveMember:      "Quitar miembro",
		SelectMemberFirst: "Seleccione primero un miembro en la lista.",
		CannotRemoveOwner: "No se puede quitar a quien es propietario.",
		ConfirmUnshare:    "¿Quitar a %s de %q?",
		SetupShared:       "Configurar localmente",
		SetupSharedTitle:  "Configurar localmente la base de datos compartida %q",
		AlreadyLocal:      "La base de datos ya está configurada localmente.",
		NewLocalPassword:  "Nueva contraseña local",
		BindExisting:      "Conectar un .kdbx existente (su propia base de datos, dispositivo nuevo)",
		BoundNowSync:      "¡Conectada! Pulse ⟳ (Sync) para traer las entradas.",
		MasterPwd:         "Contraseña maestra",
		MasterPwdFor:      "Contraseña maestra de",
		Language:          "Idioma",
		ThemeLabel:        "Tema",
		ThemeSystem:       "Sistema (seguir al SO)",
		ThemeLight:        "Claro",
		ThemeDark:         "Oscuro",
		HelpPanelLabel:    "Mostrar el panel de ayuda",
		HelpPanelDesc:     "Cuando está activado, un panel en la parte inferior de la ventana describe la pestaña en la que se encuentra: qué hace la página y a qué corresponden los botones en el programa keepass-deltasync.",

		UpdateAvailable:  "La versión %s está disponible; usted tiene la %s.",
		UpdateDownload:   "Descargar",
		UpdateCheckLabel: "Buscar actualizaciones",
		UpdateCheckDesc:  "Cuando está activado, el programa pregunta a GitLab al arrancar si existe una versión más reciente y, en tal caso, muestra una línea arriba. No se envía nada sobre usted ni sobre sus bases de datos: solo una consulta corriente a la página de versiones del proyecto.",

		HelpTitle: "Acerca de esta página",
		HelpDatabases: "## Bases de datos\n\n" +
			"Sus bases de datos y sus comparticiones. **● (círculo relleno)** = vinculada a un archivo `.kdbx` local y lista para sincronizar. **○ (círculo vacío)** = existe solo en el servidor.\n\n" +
			"**Acciones por base de datos:**\n\n" +
			"- **Sync**: enviar y recibir cambios (`sync`).\n" +
			"- **Compartir**: dar acceso a otro usuario (`share`).\n" +
			"- **Olvidar**: quitar solo el vínculo local; el servidor y el archivo quedan intactos (`forget`).\n" +
			"- **⋮ Más**: Enviar (solo subida), Recibir (solo descarga), Versiones/restaurar y **Eliminar en el servidor** (`delete-database`, definitivo para todos).\n\n" +
			"Si una base de datos existe solo en el servidor: **vincule** su propio `.kdbx` (`init --bind`) o **configure** la copia compartida (`init-shared`).\n\n" +
			"Arriba: **Añadir base de datos** (`init`) y **Sincronizar todo**.\n\n" +
			"**Órdenes de la CLI:**\n\n" +
			"- `keepass-deltasync databases`\n" +
			"- `keepass-deltasync init <name> <path>`\n" +
			"- `keepass-deltasync sync --password-stdin <db>`\n" +
			"- `keepass-deltasync push --password-stdin <db>` / `pull --password-stdin <db>`\n" +
			"- `keepass-deltasync share --password-stdin <db> <user>` / `unshare <db> <user>` / `shares <db>`\n" +
			"- `keepass-deltasync forget <db>` / `delete-database <db|uuid>`\n" +
			"- `keepass-deltasync init --bind <uuid> <db> <path>` / `init-shared --password-stdin <remote> <path>`\n" +
			"- `keepass-deltasync versions <db> <entry-uuid>` / `restore <db> <entry-uuid> <version>`",
		HelpDevices: "## Dispositivos\n\n" +
			"Los dispositivos registrados en su cuenta (`devices`). El marcado con **●** es el que está usando ahora.\n\n" +
			"- **Añadir dispositivo**: generar un token de inscripción que un dispositivo nuevo usará para registrarse (`admin user-enrollment`).\n" +
			"- **Quitar**: revocar el acceso de un dispositivo (`devices remove`). No puede quitar aquel en el que se encuentra.\n" +
			"- **Información**: ver el identificador del dispositivo y sus fechas de registro y de última actividad.\n\n" +
			"**Órdenes de la CLI:**\n\n" +
			"- `keepass-deltasync devices`\n" +
			"- `keepass-deltasync devices remove <id>`\n" +
			"- `keepass-deltasync admin user-enrollment <user>`",
		HelpActivity: "## Actividad\n\n" +
			"La salida en bruto de las llamadas a la CLI que hace la interfaz en **esta sesión** (sincronización, añadir/quitar, errores …). Se reinicia al cerrar la aplicación.\n\n" +
			"Para el historial entre sesiones use la pestaña **Registro**, que obtiene el registro de auditoría del servidor.\n\n" +
			"*No tiene una orden propia de la CLI: la pestaña solo muestra la salida de las demás órdenes.*",
		HelpLog: "## Registro\n\n" +
			"El **registro de auditoría** del servidor (`log`): un historial duradero de los eventos de su cuenta (inicio de sesión, sincronización, cambios …) en todos sus dispositivos, conservado hasta 30 días.\n\n" +
			"- **Periodo** controla cuánto se retrocede (`--since`).\n" +
			"- Cada línea muestra hora, evento, correcto/fallido, nivel e IP. Pulse **ℹ** para ver todos los detalles.\n\n" +
			"**Orden de la CLI:**\n\n" +
			"- `keepass-deltasync log --since <duration> --limit <n>`",
		HelpAdmin: "## Administración\n\n" +
			"Administración de usuarios. Requiere un **token de administración**, que se mantiene solo en memoria y nunca se guarda en disco.\n\n" +
			"- **Cargar usuarios**: listar todos los usuarios (`admin user-list`).\n" +
			"- **Crear usuario**: crearlo y obtener un token de inscripción (`admin user-create`).\n" +
			"- Por usuario: **nuevo token de inscripción** (`user-enrollment`), **activar/desactivar** (`user-enable` / `user-disable`), **eliminar** (`user-delete`, CASCADE).\n" +
			"- **Obtener token de administración (SQL)**: el SQL que crea un token nuevo (`admin token-sql`); ejecútelo en DBeaver.\n\n" +
			"**Órdenes de la CLI:**\n\n" +
			"- `keepass-deltasync admin user-list`\n" +
			"- `keepass-deltasync admin user-create <user> --display-name <name>`\n" +
			"- `keepass-deltasync admin user-enrollment <user>`\n" +
			"- `keepass-deltasync admin user-enable <user>` / `user-disable <user>`\n" +
			"- `keepass-deltasync admin user-delete <user> --yes`\n" +
			"- `keepass-deltasync admin token-sql`",
		HelpSettings: "## Ajustes\n\n" +
			"- **Idioma**: danés, inglés, alemán, francés o español.\n" +
			"- **Tema**: Sistema (seguir al SO), Claro u Oscuro.\n" +
			"- **Ruta de la CLI**: dónde está el programa `keepass-deltasync`. La interfaz lo llama para todo el trabajo.\n" +
			"- **Mostrar el panel de ayuda**: este panel.\n" +
			"- **Inicio automático**: hacer que el sistema operativo ejecute `keepass-deltasync daemon` al iniciar sesión, para que la sincronización en segundo plano funcione sin tener la interfaz abierta. Se configura en su propio ámbito de usuario (clave Run del registro en Windows, launchd en macOS, systemd --user en Linux); no hace falta administrador. La interfaz nunca ejecuta el demonio por su cuenta.\n\n" +
			"La interfaz es una envoltura sobre la línea de órdenes `keepass-deltasync`; toda la criptografía, las llamadas al servidor y la configuración viven en la CLI.\n\n" +
			"**Órdenes útiles de la CLI:**\n\n" +
			"- `keepass-deltasync status`\n" +
			"- `keepass-deltasync enroll --server <url> <token>`",
		ResetEnroll: "Restablecer el registro",

		AutostartTitle: "Inicio automático",
		AutostartDesc: "Ejecutar la sincronización en segundo plano (`keepass-deltasync daemon`) automáticamente al iniciar sesión, " +
			"sin tener esta aplicación abierta. Se configura en su propio ámbito de usuario; no hace falta administrador.",
		AutostartEnable:      "Activar el inicio automático",
		AutostartDisable:     "Desactivar el inicio automático",
		AutostartOn:          "El inicio automático está activado: el demonio arranca al iniciar sesión.",
		AutostartOff:         "El inicio automático está desactivado.",
		AutostartUnsupported: "El inicio automático no es compatible con este sistema operativo.",
		AutostartNoCLI:       "Localice primero el programa keepass-deltasync (ruta de la CLI, arriba).",
		AutostartEnabled:     "Inicio automático activado.",
		AutostartDisabled:    "Inicio automático desactivado.",

		AdminHint:         "Administración de usuarios. Requiere un token de administración (no se guarda en disco).",
		AdminNeedToken:    "Introduzca un token de administración y pulse «Cargar usuarios».",
		AdminLoadUsers:    "Cargar usuarios",
		AdminCreateUser:   "Crear usuario",
		AdminTokenHelp:    "Obtener token de administración (SQL)",
		AdminUserCount:    "%d usuario(s)",
		AdminNewEnroll:    "Nuevo token de inscripción",
		AdminEnable:       "Activar usuario",
		AdminDisable:      "Desactivar usuario",
		AdminDeleteUser:   "Eliminar usuario",
		AdminDisplayName:  "Nombre visible (opcional)",
		ConfirmDeleteUser: "¿Eliminar DEFINITIVAMENTE al usuario %q?\n\nCASCADE: se quitan todos los dispositivos, bases de datos y entradas de ese usuario. Esta acción NO se puede deshacer.",

		VersionsMenu:    "Versiones / restaurar…",
		VersionsTitle:   "Versiones — %s",
		VersionsHint:    "Pegue el UUID de una entrada para ver sus versiones conservadas (hasta 3).",
		EntryUUID:       "UUID de la entrada",
		EntryUUIDHint:   "UUID de la entrada cuyas versiones quiere ver",
		VersionsShow:    "Mostrar versiones",
		VersionsCount:   "%d versión(es)",
		VersionsNone:    "Sin versiones: no se encontró la entrada.",
		VersionsRestore: "Restaurar",
		ConfirmRestore:  "¿Restaurar la versión %s como nueva versión actual en %q?\n\nEl servidor crea una versión nueva a partir de la seleccionada. Ejecute después «Sync» (o Recibir) para traer el cambio a su archivo local.",
		RestoreDoneSync: "La versión se restauró en el servidor. Ejecute «Sync» (o Recibir) en la base de datos para traer el cambio.",
	}
}
