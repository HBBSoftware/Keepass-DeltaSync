// SPDX-License-Identifier: GPL-3.0-or-later

package main

// Français. Traduit depuis l'anglais ; l'ordre des champs suit langEN dans
// i18n.go pour que les deux restent faciles à comparer.

func init() {
	dicts[langFR] = &dict{
		AppTitle: "KeePass Delta-Sync",

		OK: "OK", Cancel: "Annuler", Close: "Fermer", Save: "Enregistrer",
		Back: "Retour", Next: "Suivant", Finish: "Terminer", Browse: "Parcourir…",
		Working: "En cours…", Done: "Terminé", Error: "Erreur",
		Copy: "Copier", DBInfoTitle: "Infos sur la base",

		CLINotFound:     "Le programme « keepass-deltasync » est introuvable.",
		CLILocate:       "Localiser keepass-deltasync",
		CLILocateHint:   "Indiquez le programme keepass-deltasync (la CLI). Il est normalement livré à côté de cette application.",
		CLIPathLabel:    "Chemin de la CLI",
		CLISelectBinary: "Sélectionner le programme keepass-deltasync",

		WizardWelcomeTitle: "Bienvenue",
		WizardWelcomeBody: "Cet assistant vous aide à synchroniser votre base KeePass " +
			"entre vos appareils.\n\nIl vous faut :\n  • L'adresse du serveur fournie par votre administrateur\n  • Un jeton d'inscription (code à usage unique)\n  • Votre fichier .kdbx",
		WizardStart:      "Démarrer l'assistant",
		WizardStepEnroll: "Étape 1 sur 2 — Inscrire l'appareil",
		WizardStepDB:     "Étape 2 sur 2 — Ajouter une base",
		ServerURL:        "Adresse du serveur",
		ServerURLHint:    "p. ex. https://deltasync.exemple.fr",
		EnrollToken:      "Jeton d'inscription",
		EnrollTokenHint:  "Le code à usage unique fourni par votre administrateur",
		DeviceName:       "Nom de l'appareil (facultatif)",
		DeviceNameHint:   "Affiché dans les listes d'administration. Vide = nom de l'ordinateur.",
		EnrollButton:     "Inscrire cet appareil",
		EnrollOK:         "Appareil inscrit !",
		WizardAddDB:      "Ajoutez votre première base",
		WizardAddDBBody:  "Enregistrez un fichier .kdbx local pour la synchronisation. Vous pourrez toujours en ajouter d'autres plus tard.",
		DBName:           "Nom",
		DBNameHint:       "Un nom court, p. ex. « perso » ou « travail »",
		KdbxFile:         "Fichier KeePass (.kdbx)",
		KdbxFileHint:     "Chemin de votre base locale",
		CreateDBButton:   "Créer la base",
		SkipForNow:       "Passer pour l'instant",
		WizardDone:       "Tout est prêt !",
		WizardDoneBody:   "Vous pouvez synchroniser. Utilisez le bouton « Sync » sur une base pour envoyer et récupérer les modifications.",

		WizardAdvanced:     "Avancé (administrateur)",
		WizardFirefox:      "Rechercher depuis Firefox — sans compte",
		FFIntro:            "Cherchez dans vos entrées KeePass depuis Firefox et allez droit au bon site. Ni compte ni serveur nécessaires — seulement deux choses, une fois : indiquer votre .kdbx au programme, et l'enregistrer auprès de Firefox. Le remplissage des identifiants reste l'affaire de KeePassXC-Browser ; l'extension ne voit jamais de mot de passe.",
		FFAddLocal:         "Enregistrer une base…",
		FFAddLocalTitle:    "Enregistrer une base pour la recherche",
		FFAddLocalDone:     "La base est enregistrée. Ensuite : Enregistrer auprès de Firefox — puis redémarrez Firefox.",
		FFSavePassword:     "Conserver le mot de passe maître dans le trousseau du système (pour que la fenêtre ne le demande pas)",
		FFPasswordMissing:  "Saisissez le mot de passe maître, ou décochez la case — l'extension le demandera alors elle-même.",
		FFInstallHost:      "Enregistrer auprès de Firefox",
		FFHostInstalled:    "Enregistré auprès de Firefox",
		FFRestartFirefox:   "Redémarrez Firefox — c'est ce qui lui fait prendre l'enregistrement en compte.",
		FFPreview:          "Afficher ce qui serait écrit",
		FFPreviewTitle:     "Simulation (rien n'est écrit)",
		FFUninstallHost:    "Supprimer l'enregistrement",
		FFUninstallConfirm: "Supprimer l'enregistrement de toutes les variantes de Firefox présentes sur cette machine ? Ni la base ni le fichier ne sont touchés.",
		FFGuide:            "Guide",
		FFProbe:            "Tester",
		FFProbeTitle:       "Ce que le navigateur recevrait — %s",
		FFProbePwdHint:     "Facultatif — repris du trousseau s'il y est enregistré",
		FFProbeEmpty:       "L'index est vide : aucune entrée avec une URL exploitable.",
		FFCount:            "%d base(s) consultable(s) depuis Firefox",
		FFNone:             "Aucune base enregistrée pour l'instant — commencez par Enregistrer une base.",
		FFKindSynced:       "(synchronisée)",
		FFKindLocal:        "(locale uniquement)",
		FFCLIPath:          "Programme :",
		HelpFirefox: "## Firefox\n\n" +
			"Recherche en texte libre dans vos entrées depuis la barre d'adresse de Firefox (`kp` + espace) ou depuis la fenêtre (**Alt+Maj+K**), et une touche ouvre le site de l'entrée. L'extension reçoit **uniquement** le titre, les URL et le chemin du groupe — jamais de mots de passe, de noms d'utilisateur, de notes ni de pièces jointes.\n\n" +
			"La configuration tient en deux étapes, une fois par machine :\n\n" +
			"- **Enregistrer une base** — indique un `.kdbx` au programme (`add-local`). Aucun serveur, aucun compte, rien n'est envoyé. Cochez la case et le mot de passe maître va dans le trousseau du système, afin que la fenêtre ne le demande pas.\n" +
			"- **Enregistrer auprès de Firefox** — écrit le manifeste de messagerie native pour que Firefox ait le droit de lancer l'hôte (`install-browser-host`). **Redémarrez Firefox ensuite.**\n\n" +
			"En plus : **Tester** montre l'index exact que le navigateur recevrait (`browser-host --probe`), et **Afficher ce qui serait écrit** effectue le même enregistrement sans rien modifier (`--dry-run`).\n\n" +
			"Le module lui-même s'installe depuis addons.mozilla.org — voir **Guide**.\n\n" +
			"**Commandes CLI :**\n\n" +
			"- `keepass-deltasync add-local <name> <file.kdbx>`\n" +
			"- `keepass-deltasync install-browser-host`\n" +
			"- `keepass-deltasync browser-host --probe <name>`\n" +
			"- `keepass-deltasync uninstall-browser-host`",
		WizardStepAdvanced: "Inscription avancée — administrateur",
		AdvancedIntro: "Si vous disposez d'un jeton d'administration, vous pouvez émettre un jeton d'inscription et inscrire ce PC en une seule étape — " +
			"aucun jeton n'est nécessaire au préalable. Choisissez un utilisateur existant, ou créez-en un.",
		AdvUserMode:     "Utilisateur",
		AdvExistingUser: "Utilisateur existant",
		AdvNewUser:      "Créer un utilisateur",
		AdvEnrollButton: "Émettre un jeton et inscrire ce PC",
		AdvIssuingToken: "Émission du jeton d'inscription…",
		AdvEnrolling:    "Inscription de ce PC…",
		AdvNoTokenErr:   "Impossible de lire le jeton d'inscription dans la réponse du serveur. Voir le journal pour les détails.",

		TabDatabases: "Bases",
		TabDevices:   "Appareils",
		TabFirefox:   "Firefox",
		TabActivity:  "Activité",
		TabLog:       "Journal",
		TabAdmin:     "Administration",
		TabSettings:  "Réglages",
		ActivityHint: "La sortie des appels à la CLI (sync, ajout/suppression, erreurs …) s'affiche ici.",
		Clear:        "Effacer",

		LogHint:         "Journal d'activité du serveur (audit) — l'historique sur tous les appareils, conservé jusqu'à 30 jours.",
		LogCount:        "%d entrées de journal",
		LogEmpty:        "(aucune entrée de journal sur cette période)",
		LogPeriodLabel:  "Période :",
		LogPeriod24h:    "Dernières 24 heures",
		LogPeriod7d:     "7 derniers jours",
		LogPeriod30d:    "30 derniers jours",
		LogPeriodAll:    "Tout",
		LogDetailsTitle: "Détails du journal",
		LogColTime:      "Heure",
		LogColEvent:     "Événement",
		LogColLevel:     "Niveau",
		LogColIP:        "Adresse IP",
		LogOK:           "OK",
		LogFail:         "Échec",

		ColEnrolled:     "Inscrit",
		ColLastSeen:     "Vu pour la dernière fois",
		ThisDevice:      "● cet appareil",
		DevCount:        "%d appareil(s) sur le compte",
		DeviceInfoTitle: "Infos sur l'appareil",

		AddDevice:           "Ajouter un appareil",
		AddDeviceCreate:     "Générer un jeton",
		RemoveDevice:        "Retirer l'appareil",
		Username:            "Nom d'utilisateur",
		AdminToken:          "Jeton d'administration",
		EnrollTokenCreated:  "Jeton d'inscription pour le nouvel appareil",
		SelectDeviceFirst:   "Sélectionnez d'abord un appareil dans la liste.",
		CannotRemoveCurrent: "Vous ne pouvez pas retirer d'ici l'appareil que vous utilisez actuellement.",
		ConfirmRemoveDevice: "Retirer (révoquer) l'appareil %q ? Son jeton deviendra invalide.",
		StatusBox:           "État",
		NotEnrolled:         "Pas encore inscrit.",
		Refresh:             "Actualiser",
		Sync:                "Sync",
		SyncSelected:        "Synchroniser la sélection",
		SyncAll:             "Tout synchroniser",
		SelectFirst:         "Sélectionnez d'abord une base dans la liste.",
		AddDatabase:         "Ajouter une base",
		ForgetDatabase:      "Oublier la base",
		ConfirmForget:       "Oublier le lien local vers %q ? Le fichier .kdbx et la base sur le serveur ne sont PAS touchés — seul le lien dans ce client est supprimé.",
		MoreActions:         "Autres actions",
		PushNow:             "Pousser maintenant (envoi seul)",
		PullNow:             "Tirer maintenant (réception seule)",
		DeleteOnServer:      "Supprimer sur le serveur",
		ConfirmDeleteServer: "Supprimer DÉFINITIVEMENT la base %q sur le serveur ?\n\nCela retire TOUTES les entrées, versions, partages et l'historique — pour tous les utilisateurs. Cette action est IRRÉVERSIBLE. Votre fichier .kdbx local reste intact.",
		NoDatabases:         "Aucune base pour l'instant. Cliquez sur « Ajouter une base » pour commencer.",
		DBCount:             "%d base(s) connectée(s)",
		ColName:             "Nom",
		ColStatus:           "État",
		ColID:               "ID",
		ColCreated:          "Créée",
		ColPath:             "Chemin local",
		BoundLocally:        "● prête",
		OnServerOnly:        "○ serveur uniquement",

		MembersTitle:       "Connectés à la base",
		MembersOf:          "Connectés à %q",
		SelectToSeeMembers: "Sélectionnez une base ci-dessus pour voir qui y est connecté.",
		MemberCount:        "%d membre(s)",
		MembersNeedBound:   "La base doit être configurée localement pour afficher les membres.",
		MembersUnavailable: "Impossible de récupérer les membres (seul le propriétaire peut les voir) :",
		NoMembers:          "Vous seulement — pas encore partagée.",
		ColRole:            "Rôle",
		ColUser:            "Nom d'utilisateur",
		ColDisplay:         "Nom affiché",
		ColAdded:           "Ajouté",

		ShareDatabase:     "Partager la base",
		ShareTitle:        "Partager %q avec un utilisateur",
		ShareWith:         "Partager avec (nom d'utilisateur)",
		RemoveMember:      "Retirer le membre",
		SelectMemberFirst: "Sélectionnez d'abord un membre dans la liste.",
		CannotRemoveOwner: "Le propriétaire ne peut pas être retiré.",
		ConfirmUnshare:    "Retirer %s de %q ?",
		SetupShared:       "Configurer localement",
		SetupSharedTitle:  "Configurer localement la base partagée %q",
		AlreadyLocal:      "La base est déjà configurée localement.",
		NewLocalPassword:  "Nouveau mot de passe local",
		BindExisting:      "Relier un .kdbx existant (votre propre base, nouvel appareil)",
		BoundNowSync:      "Reliée ! Cliquez sur ⟳ (Sync) pour récupérer les entrées.",
		MasterPwd:         "Mot de passe maître",
		MasterPwdFor:      "Mot de passe maître pour",
		Language:          "Langue",
		ThemeLabel:        "Thème",
		ThemeSystem:       "Système (suivre l'OS)",
		ThemeLight:        "Clair",
		ThemeDark:         "Sombre",
		HelpPanelLabel:    "Afficher le panneau d'aide",
		HelpPanelDesc:     "Lorsque c'est activé, un panneau en bas de la fenêtre décrit l'onglet où vous êtes — ce que fait la page, et à quoi les boutons correspondent dans le programme keepass-deltasync.",

		UpdateAvailable:  "La version %s est disponible — vous utilisez la %s.",
		UpdateDownload:   "Télécharger",
		UpdateCheckLabel: "Rechercher les mises à jour",
		UpdateCheckDesc:  "Lorsque c'est activé, le programme demande à GitLab au démarrage s'il existe une version plus récente, et affiche le cas échéant une ligne en haut. Rien sur vous ni sur vos bases n'est envoyé — juste une consultation ordinaire de la page des versions du projet.",

		HelpTitle: "À propos de cette page",
		HelpDatabases: "## Bases\n\n" +
			"Vos bases et leurs partages. **● (cercle plein)** = reliée à un fichier `.kdbx` local et prête à synchroniser. **○ (cercle vide)** = n'existe que sur le serveur.\n\n" +
			"**Actions par base :**\n\n" +
			"- **Sync** — envoyer et récupérer les modifications (`sync`).\n" +
			"- **Partager** — donner accès à un autre utilisateur (`share`).\n" +
			"- **Oublier** — supprimer uniquement le lien local ; le serveur et le fichier restent intacts (`forget`).\n" +
			"- **⋮ Plus** — Pousser (envoi seul), Tirer (réception seule), Versions/restauration, et **Supprimer sur le serveur** (`delete-database`, définitif pour tout le monde).\n\n" +
			"Si une base n'existe que sur le serveur : **reliez** votre propre `.kdbx` (`init --bind`) ou **configurez** la copie partagée (`init-shared`).\n\n" +
			"En haut : **Ajouter une base** (`init`) et **Tout synchroniser**.\n\n" +
			"**Commandes CLI :**\n\n" +
			"- `keepass-deltasync databases`\n" +
			"- `keepass-deltasync init <name> <path>`\n" +
			"- `keepass-deltasync sync --password-stdin <db>`\n" +
			"- `keepass-deltasync push --password-stdin <db>` / `pull --password-stdin <db>`\n" +
			"- `keepass-deltasync share --password-stdin <db> <user>` / `unshare <db> <user>` / `shares <db>`\n" +
			"- `keepass-deltasync forget <db>` / `delete-database <db|uuid>`\n" +
			"- `keepass-deltasync init --bind <uuid> <db> <path>` / `init-shared --password-stdin <remote> <path>`\n" +
			"- `keepass-deltasync versions <db> <entry-uuid>` / `restore <db> <entry-uuid> <version>`",
		HelpDevices: "## Appareils\n\n" +
			"Les appareils inscrits sur votre compte (`devices`). Celui qui porte le **●** est l'appareil que vous utilisez en ce moment.\n\n" +
			"- **Ajouter un appareil** — générer un jeton d'inscription qu'un nouvel appareil utilisera pour s'inscrire (`admin user-enrollment`).\n" +
			"- **Retirer** — révoquer l'accès d'un appareil (`devices remove`). Vous ne pouvez pas retirer celui que vous utilisez.\n" +
			"- **Infos** — voir l'identifiant de l'appareil ainsi que ses dates d'inscription et de dernière activité.\n\n" +
			"**Commandes CLI :**\n\n" +
			"- `keepass-deltasync devices`\n" +
			"- `keepass-deltasync devices remove <id>`\n" +
			"- `keepass-deltasync admin user-enrollment <user>`",
		HelpActivity: "## Activité\n\n" +
			"La sortie brute des appels à la CLI que l'interface effectue dans **cette session** (sync, ajout/suppression, erreurs …). Elle est remise à zéro à la fermeture de l'application.\n\n" +
			"Pour l'historique d'une session à l'autre, utilisez l'onglet **Journal**, qui récupère le journal d'audit du serveur.\n\n" +
			"*Pas de commande CLI propre — l'onglet se contente d'afficher la sortie des autres commandes.*",
		HelpLog: "## Journal\n\n" +
			"Le **journal d'audit** du serveur (`log`) — un historique durable des événements de votre compte (connexion, synchronisation, modifications …), sur tous vos appareils, conservé jusqu'à 30 jours.\n\n" +
			"- **Période** détermine jusqu'où remonter (`--since`).\n" +
			"- Chaque ligne indique l'heure, l'événement, OK/échec, le niveau et l'IP. Cliquez sur **ℹ** pour tous les détails.\n\n" +
			"**Commande CLI :**\n\n" +
			"- `keepass-deltasync log --since <duration> --limit <n>`",
		HelpAdmin: "## Administration\n\n" +
			"Administration des utilisateurs. Nécessite un **jeton d'administration**, conservé uniquement en mémoire et jamais écrit sur le disque.\n\n" +
			"- **Charger les utilisateurs** — lister tous les utilisateurs (`admin user-list`).\n" +
			"- **Créer un utilisateur** — le créer et obtenir un jeton d'inscription (`admin user-create`).\n" +
			"- Par utilisateur : **nouveau jeton d'inscription** (`user-enrollment`), **activer/désactiver** (`user-enable` / `user-disable`), **supprimer** (`user-delete`, CASCADE).\n" +
			"- **Obtenir un jeton d'administration (SQL)** — le SQL qui crée un nouveau jeton d'administration (`admin token-sql`) ; exécutez-le dans DBeaver.\n\n" +
			"**Commandes CLI :**\n\n" +
			"- `keepass-deltasync admin user-list`\n" +
			"- `keepass-deltasync admin user-create <user> --display-name <name>`\n" +
			"- `keepass-deltasync admin user-enrollment <user>`\n" +
			"- `keepass-deltasync admin user-enable <user>` / `user-disable <user>`\n" +
			"- `keepass-deltasync admin user-delete <user> --yes`\n" +
			"- `keepass-deltasync admin token-sql`",
		HelpSettings: "## Réglages\n\n" +
			"- **Langue** — danois, anglais, allemand, français ou espagnol.\n" +
			"- **Thème** — Système (suivre l'OS), Clair ou Sombre.\n" +
			"- **Chemin de la CLI** — l'emplacement du programme `keepass-deltasync`. L'interface l'appelle pour tout le travail.\n" +
			"- **Afficher le panneau d'aide** — ce panneau.\n" +
			"- **Démarrage automatique** — faire exécuter `keepass-deltasync daemon` par le système à l'ouverture de session, pour que la synchronisation de fond tourne sans que l'interface soit ouverte. Se configure dans votre propre portée utilisateur (clé Run du registre sous Windows, launchd sous macOS, systemd --user sous Linux) — aucun administrateur requis. L'interface ne lance jamais le démon elle-même.\n\n" +
			"L'interface n'est qu'une enveloppe autour de la ligne de commande `keepass-deltasync` ; toute la cryptographie, les appels au serveur et la configuration vivent dans la CLI.\n\n" +
			"**Commandes CLI utiles :**\n\n" +
			"- `keepass-deltasync status`\n" +
			"- `keepass-deltasync enroll --server <url> <token>`",
		ResetEnroll: "Réinitialiser l'inscription",

		AutostartTitle: "Démarrage automatique",
		AutostartDesc: "Exécuter la synchronisation de fond (`keepass-deltasync daemon`) automatiquement à l'ouverture de session, " +
			"sans que cette application soit ouverte. Se configure dans votre propre portée utilisateur — aucun administrateur requis.",
		AutostartEnable:      "Activer le démarrage automatique",
		AutostartDisable:     "Désactiver le démarrage automatique",
		AutostartOn:          "Le démarrage automatique est actif — le démon se lance à l'ouverture de session.",
		AutostartOff:         "Le démarrage automatique est inactif.",
		AutostartUnsupported: "Le démarrage automatique n'est pas pris en charge sur ce système d'exploitation.",
		AutostartNoCLI:       "Localisez d'abord le programme keepass-deltasync (chemin de la CLI ci-dessus).",
		AutostartEnabled:     "Démarrage automatique activé.",
		AutostartDisabled:    "Démarrage automatique désactivé.",

		AdminHint:         "Administration des utilisateurs. Nécessite un jeton d'administration (non enregistré sur le disque).",
		AdminNeedToken:    "Saisissez un jeton d'administration et cliquez sur « Charger les utilisateurs ».",
		AdminLoadUsers:    "Charger les utilisateurs",
		AdminCreateUser:   "Créer un utilisateur",
		AdminTokenHelp:    "Obtenir un jeton d'administration (SQL)",
		AdminUserCount:    "%d utilisateur(s)",
		AdminNewEnroll:    "Nouveau jeton d'inscription",
		AdminEnable:       "Activer l'utilisateur",
		AdminDisable:      "Désactiver l'utilisateur",
		AdminDeleteUser:   "Supprimer l'utilisateur",
		AdminDisplayName:  "Nom affiché (facultatif)",
		ConfirmDeleteUser: "Supprimer DÉFINITIVEMENT l'utilisateur %q ?\n\nCASCADE : tous les appareils, bases et entrées de cet utilisateur sont supprimés. Cette action est IRRÉVERSIBLE.",

		VersionsMenu:    "Versions / restauration…",
		VersionsTitle:   "Versions — %s",
		VersionsHint:    "Collez l'UUID d'une entrée pour voir les versions conservées (jusqu'à 3).",
		EntryUUID:       "UUID de l'entrée",
		EntryUUIDHint:   "UUID de l'entrée dont vous voulez les versions",
		VersionsShow:    "Afficher les versions",
		VersionsCount:   "%d version(s)",
		VersionsNone:    "Aucune version — entrée introuvable.",
		VersionsRestore: "Restaurer",
		ConfirmRestore:  "Restaurer la version %s comme nouvelle version courante dans %q ?\n\nLe serveur crée une nouvelle version à partir de celle sélectionnée. Lancez ensuite « Sync » (ou Tirer) pour faire descendre la modification dans votre fichier local.",
		RestoreDoneSync: "La version a été restaurée sur le serveur. Lancez « Sync » (ou Tirer) sur la base pour faire descendre la modification.",
	}
}
