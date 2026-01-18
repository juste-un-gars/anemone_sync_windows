# État des Sessions - SMBSync

Ce fichier contient un résumé très court de chaque session de développement.

---

## Session 001 - 2026-01-11
**Status**: ✅ Terminée
**Durée**: ~2h
**Phase**: Phase 0 complète + Infrastructure professionnelle
**Réalisations**: 35 fichiers, ~5400 lignes (Go: 800, SQL: 200, Config: 900, Docs: 3500)
**Infrastructure**: CI/CD 6 jobs, Makefile 15+ commandes, golangci-lint, Dependabot, templates GitHub
**Documentation**: README, INSTALLATION, CONTRIBUTING, SECURITY, CODE_OF_CONDUCT, INSTALLER, CHECKLIST
**Commits**: 4 (Phase 0, LICENSE, Infrastructure, Checklist)
**Détails**: sessions/session_001.md

## Session 002 - 2026-01-11
**Status**: ✅ Terminée (commité)
**Durée**: ~3h
**Phase**: Phase 1 Scanner (code + tests)
**Réalisations**: 15 fichiers, ~4100 lignes (Scanner: 1600, Tests: 2500)
**Modules**: errors, metadata, hash, exclusion, walker, worker, scanner (7/7 ✅)
**Tests**: 65+ tests unitaires, intégration, benchmarks créés
**DB Extensions**: 7 méthodes ajoutées pour scanner
**Commit**: 1929806 "feat(scanner): Implement Phase 1 File Scanner"
**Architecture Décidée**: OneDrive-like avec client SMB intégré (Option B)
**Prochaines étapes**: Phase 2 Client SMB + Cache Intelligent
**Détails**: sessions/session_002.md

## Session 003 - 2026-01-11
**Status**: ✅ Terminée (commité)
**Durée**: ~2h
**Phase**: Phase 1 Scanner (finalisation tests + DB fixes)
**Réalisations**: 97% tests passent (61/63), coverage 73%
**Corrections**: Worker pool deadlock, DB timestamps, NULL handling, types time.Time→int64
**Tests**: Worker pool 13/13 ✅, Hash 13/13 ✅, Walker 11/11 ✅
**Bugs Résolés**: 8 bugs majeurs (deadlock, constraints, type mismatch, NULL scan)
**Commit**: f360853 "test(scanner): Fix worker pool tests and database issues"
**Restant**: 2 tests (exclusions + cancellation context)
**Prochaines étapes**: Finaliser 2 tests restants (~30 min) puis Phase 2 Client SMB
**Détails**: sessions/session_003.md

## Session 004 - 2026-01-11
**Status**: ✅ Terminée (commité)
**Durée**: ~30 min
**Phase**: Phase 1 Scanner (100% complète ✅)
**Réalisations**: 100% tests passent (63/63 ✅)
**Corrections**:
- Default exclusions loading (multi-path search)
- Context cancellation error propagation in Walker
**Tests**: Tous les tests passent, Phase 1 100% fonctionnelle
**Fichiers modifiés**: scanner.go, walker.go
**Commit**: dad73a1 "fix(scanner): Fix remaining tests - Phase 1 complete"
**Prochaines étapes**: Phase 2 Client SMB + Cache Intelligent

## Session 005 - 2026-01-11
**Status**: ✅ Terminée (commité)
**Durée**: ~1h
**Phase**: Phase 2 Client SMB (Paliers 1-2)
**Réalisations**:
- **Palier 1**: Connection management (Connect/Disconnect/IsConnected)
- **Palier 2**: File operations (Download/Upload avec auto-création dossiers)
**Dependencies**: go-smb2 v1.1.0, golang.org/x/crypto v0.16.0
**Tests**: 12/12 passent ✅
**Fichiers créés**: internal/smb/client.go (326 lignes), client_test.go (225 lignes)
**Commit**: 9c5b175 "feat(smb): Add SMB client with Download/Upload"
**Restant**: Palier 3 (ListRemote, GetMetadata, Delete)
**Prochaines étapes**: Paliers 3-4 puis authentification sécurisée (keyring)

## Session 006 - 2026-01-12
**Status**: ✅ Terminée (commité)
**Durée**: ~2h
**Phase**: Phase 2 Client SMB (Paliers 3-4 - Complet ✅) + Phase 3 Cache Intelligent (Complet ✅)
**Réalisations**:
- **Phase 2 Palier 3**: ListRemote, GetMetadata, Delete + RemoteFileInfo structure
- **Phase 2 Palier 4**: Keyring auth (CredentialManager, NewSMBClientFromKeyring, Save/Delete)
- **Phase 3 Cache**: CacheManager + ChangeDetector (3-way merge, conflict resolution)
**Tests**: 34/34 passent ✅ (SMB: 23, Cache: 11)
**Fichiers créés**:
  - Phase 2: credentials.go (152), credentials_test.go (210), client.go (+196), client_test.go (+163)
  - Phase 3: cache.go (264), detector.go (290), cache_test.go (241), detector_test.go (301)
**Commits**: 5 (Palier 3: 2aaf5ae, Palier 4: d487d0e, Docs: 7776f6b/d72fef6, Cache: e4550cd)
**Phase 2 & 3 Complètes**: Client SMB + Cache intelligent 3-way merge
**Prochaines étapes**: Phase 4 Moteur de Synchronisation

## Session 007 - 2026-01-13
**Status**: ✅ Terminée (Palier 1 commité)
**Durée**: ~2h
**Phase**: Phase 4 Moteur de Synchronisation (Palier 1/4)
**Approche**: Implémentation par paliers progressifs (mode mirror bidirectionnel)
**Réalisations**:
- **Types & Structures**: types.go (SyncRequest, SyncResult, SyncAction, SyncProgress, modes, status)
- **Engine**: engine.go (~570 lignes) - Orchestrateur principal avec cycle 5 phases
- **Executor**: executor.go (~330 lignes) - Exécution séquentielle d'actions (upload/download/delete)
- **Errors**: errors.go (~330 lignes) - Classification erreurs (transient/permanent, network/fs/smb)
- **DB Extensions**: GetSyncJob, UpdateJobStatus, UpdateJobLastRun, InsertSyncHistory, GetJobStatistics
**Architecture**: 5 phases (Préparation → Scanning → Détection → Exécution → Finalisation)
**Intégrations**: Scanner (Phase 1), SMB (Phase 2), Cache+Detector (Phase 3)
**Total**: 4 fichiers, ~1826 lignes, compile ✅
**Commit**: a13353b "feat(sync): Implement Phase 4 Palier 1 - Sync Engine Foundation"
**Prochaines étapes**: Palier 2 (remote_scanner + progress), puis Palier 3 (retry + conflict), Palier 4 (worker pool + tests)

## Session 008 - 2026-01-13
**Status**: ✅ Terminée (Palier 2 commité)
**Durée**: ~1.5h
**Phase**: Phase 4 Moteur de Synchronisation (Palier 2/4)
**Réalisations**:
- **RemoteScanner**: remote_scanner.go (230 lignes) - Scan récursif SMB avec callbacks, gestion erreurs partielles
- **ProgressTracker**: progress.go (260 lignes) - Calcul pourcentage automatique, throttling, transfer rate/ETA
- **SMBClientInterface**: Interface pour mock/test, découplage dépendances
- **Tests**: remote_scanner_test.go (365 lignes), progress_test.go (485 lignes) - 24 tests, 100% passent ✅
- **Intégration**: engine.go mis à jour pour utiliser RemoteScanner au lieu de ListRemote simple
**Features**: Progress callbacks (10 dirs/100 fichiers), cancellation, partial success, phase weights
**Total**: 4 fichiers créés, 1 modifié, ~1393 lignes ajoutées
**Commit**: 22de4af "feat(sync): Implement Phase 4 Palier 2 - Remote Scanner & Progress System"
**Prochaines étapes**: Palier 3 (retry logic + conflict resolution), puis Palier 4 (worker pool + tests complets)

## Session 009 - 2026-01-13
**Status**: ✅ Terminée (Palier 3 commité)
**Durée**: ~2h
**Phase**: Phase 4 Moteur de Synchronisation (Palier 3/4)
**Réalisations**:
- **Retry System**: retry.go (275 lignes) - Exponential backoff, jitter, policies (default/aggressive/none)
- **Conflict Resolution**: conflict_resolver.go (265 lignes) - 4 stratégies (recent/local/remote/ask)
- **Integration Executor**: Retry automatique pour upload/download/delete avec classification erreurs
- **Integration Engine**: Résolution conflits automatique dans detectChanges phase
- **Tests**: retry_test.go (360 lignes), conflict_resolver_test.go (395 lignes) - 41 tests, 100% passent ✅
**Features**: Context cancellation, callback retries, tiebreakers (size), logging complet
**Total**: 4 fichiers créés, 2 modifiés, ~1488 lignes ajoutées
**Commit**: fea5e1e "feat(sync): Implement Phase 4 Palier 3 - Retry Logic & Conflict Resolution"
**Prochaines étapes**: Palier 4 (worker pool parallèle + tests d'intégration complets)

## Session 010 - 2026-01-13
**Status**: ✅ Terminée (Palier 4 & Phase 4 COMPLÈTE ✅)
**Durée**: ~2.5h
**Phase**: Phase 4 Moteur de Synchronisation (Palier 4/4 - FINAL)
**Réalisations**:
- **WorkerPool**: worker_pool.go (350 lignes) - Pool configurable, job distribution, result collection atomic
- **Parallel Execution**: ExecuteParallel + SetParallelMode pour switch sequential/parallel
- **Integration Tests**: integration_test.go (380 lignes) - Engine creation, validation, error handling
- **Worker Pool Tests**: worker_pool_test.go (410 lignes) - 13 tests lifecycle, jobs, cancellation
- **Executor Integration**: Mode parallèle transparent avec fallback séquentiel
- **Bug Fix**: Context cancellation race condition dans Submit (235a500)
**Features**: Context cancellation, statistics atomiques, channels bufferisés, ordering preservation
**Total**: 3 fichiers créés, 1 modifié, ~1140 lignes ajoutées
**Tests**: 71+ tests Phase 4 (Paliers 1-4), tous passent ✅
**Commits**: cf3da27 "feat(sync): Implement Phase 4 Palier 4 - Worker Pool & Integration Tests", 235a500 "fix(sync): Fix context cancellation check"
**PHASE 4 COMPLÈTE**: Engine complet (orchestration + remote scan + retry + conflict + worker pool)
**Prochaines étapes**: Phase 5 Interface CLI (init, add, start, stop, status) ou Phase 6 Watchers temps réel

## Session 011 - 2026-01-14
**Status**: ✅ Terminée (Documentation complète)
**Durée**: ~1h
**Phase**: Documentation Sessions 004-010 + ARCHITECTURE.md
**Réalisations**:
- **Sessions détaillées**: session_004.md à session_010.md (7 fichiers créés)
- **ARCHITECTURE.md**: Mise à jour complète avec Phases 0-4 (détails modules, tests, performance)
- **Couverture**: Paliers détaillés Phase 4, décisions techniques, tests, bugs fixes
- **État projet**: 15000+ lignes, 150+ tests, 75-80% coverage, production-ready backend ✅
**Total**: 7 sessions documentées (~8000 lignes docs), 1 ARCHITECTURE.md mis à jour
**Prochaines étapes**: Phase 5 CLI (cobra, commandes, progress bars) ou Phase 6 Watchers (fsnotify, background sync)
**Détails**: Toutes les sessions 004-010 dans sessions/

## Session 012 - 2026-01-14
**Status**: ✅ Terminée (code Palier 1 complet)
**Durée**: ~1h
**Phase**: Phase 5 Application Desktop - Palier 1 (Fyne + System Tray)
**Décision Architecture**: App utilisateur (pas service Windows) avec auto-start, comme OneDrive/Dropbox
**GUI Framework**: Fyne (robuste, pure Go, cross-platform, pérenne)
**Réalisations**:
- **app.go** (~180 lignes): Lifecycle, context cancellation, état syncing
- **tray.go** (~100 lignes): System tray menu (Status, Sync Now, Settings, Quit)
- **icon.go** (~40 lignes): Icône PNG embedded (teal placeholder)
- **main.go** (~50 lignes): Entry point avec zap logger
**Dépendances ajoutées**: fyne.io/fyne/v2 v2.7.2, fyne.io/systray v1.12.0
**Binaire**: anemonesync.exe (~24MB) compile ✅
**Prochaines étapes**: Session 013 - résoudre problème compilateur CGO

## Session 013 - 2026-01-14
**Status**: ✅ Terminée (CGO fix)
**Durée**: ~45min
**Phase**: Phase 5 - Fix compilation CGO/Fyne
**Problème résolu**: TDM-GCC 10.3.0 incompatible avec Go 1.25.5 pour CGO
- Erreur Windows: "%1 n'est pas une application Win32 valide"
- Binaires compilés corrompus malgré en-tête PE valide
**Solution**: MSYS2 MinGW64 GCC 15.2.0 (PATH: /c/msys64/mingw64/bin)
**Résultat**: anemonesync.exe 22MB fonctionne ✅ (logs zap, system tray OK)
**Prochaines étapes**: Phase 5 Palier 2 (Settings window, sync jobs UI, status bar)

## Session 014 - 2026-01-14
**Status**: ✅ Terminée (Palier 2 complet)
**Durée**: ~30min
**Phase**: Phase 5 Application Desktop - Palier 2 (Settings UI)
**Réalisations**:
- **settings.go** (~180 lignes): Fenêtre settings avec 3 tabs (Jobs, General, About)
- **types.go** (~100 lignes): SyncJob, JobStatus, AppSettings
- **joblist.go** (~200 lignes): Liste des sync jobs avec status indicators colorés
- **jobform.go** (~310 lignes): Formulaire création/édition job (SMB, credentials, mode, schedule)
- **app.go** (+180 lignes): Gestion jobs (CRUD), settings, credentials
**Features**: Tabs navigation, status colors (green/blue/orange/red), folder browser, password field
**Total**: 5 fichiers, ~970 lignes ajoutées
**Binaire**: anemonesync.exe 52MB compile et exécute ✅
**Prochaines étapes**: Phase 5 Palier 3 (Persistence DB, auto-start Windows, notifications)

## Session 015 - 2026-01-14
**Status**: ✅ Terminée (Palier 3 complet)
**Durée**: ~45min
**Phase**: Phase 5 Application Desktop - Palier 3 (Persistence & Services)
**Réalisations**:
- **db.go** (+220 lignes): CRUD sync_jobs (GetAll, Create, Update, Delete), AppConfig (Get/Set)
- **autostart.go** (~75 lignes): Windows registry auto-start (HKCU\...\Run)
- **notifications.go** (~110 lignes): Système notifications Fyne (sync start/complete/fail/conflict)
- **app.go** (+200 lignes): Intégration DB, AutoStart, Notifier, CredentialManager
- **Conversions**: DB ↔ App types (SyncJob, schedules, remote paths)
**Features**: Persistence SQLite chiffrée, auto-start Windows, notifications toast, credentials keyring
**Total**: 4 fichiers créés/modifiés, ~605 lignes ajoutées
**Binaire**: anemonesync.exe 53MB compile et exécute ✅ (DB loading OK)
**Prochaines étapes**: Phase 5 Palier 4 (Scheduler background, file watchers) ou Phase 6 (Integration sync engine)

## Session 016 - 2026-01-14
**Status**: ✅ Terminée (Palier 4 complet)
**Durée**: ~45min
**Phase**: Phase 5 Application Desktop - Palier 4 (Scheduler & File Watchers)
**Réalisations**:
- **scheduler.go** (~220 lignes): Scheduler périodique avec timers par job (5m/15m/30m/1h)
- **watcher.go** (~280 lignes): File watcher fsnotify avec debouncing (3s), recursive, ignore temp files
- **syncmanager.go** (~250 lignes): Coordonne sync.Engine + notifications + progress callbacks
- **app.go** (+100 lignes): Intégration startWorkers/shutdown, ExecuteJobSync, update job CRUD
- **settings.go** (+6 lignes): RefreshJobList() pour UI updates
**Features**: Debouncing, context cancellation, auto-watch nouveaux dossiers, reschedule dynamique
**Dépendance ajoutée**: github.com/fsnotify/fsnotify
**Total**: 3 fichiers créés, 2 modifiés, ~750 lignes ajoutées
**Binaire**: anemonesync.exe 56MB compile ✅, tests sync passent ✅
**PHASE 5 COMPLÈTE**: App Desktop fonctionnelle (UI + persistence + scheduler + watchers + sync engine)
**Prochaines étapes**: Tests manuels, polish UI, ou Phase 6 (watchers SMB remote)

## Session 017 - 2026-01-15
**Status**: ⚠️ Partiellement terminée (bug UI à corriger)
**Durée**: ~1h
**Phase**: Phase 6 - Remote SMB Watcher (bidirectionnel complet)
**Réalisations**:
- **remote_watcher.go** (~430 lignes): Polling SMB périodique, snapshots légers (count+bytes), détection changements
- **types.go**: Ajout RemoteWatch + RemotePollInterval au SyncJob
- **jobform.go**: UI checkbox "Watch remote" + select intervalle (30s/1m/5m)
- **app.go**: Intégration RemoteWatcher (start/stop/rewatch dans lifecycle)
**Features**: Polling configurable par job, snapshot comparison, context cancellation, keyring auth
**Total**: 1 fichier créé, 3 modifiés, ~500 lignes ajoutées
**Binaire**: anemonesync.exe compile ✅, tests passent ✅

**⚠️ BUG NON RÉSOLU**: Fenêtre Settings ne s'ouvre pas (erreur thread Fyne depuis systray)
- Erreur: "Error in Fyne call thread, this should have been called in fyne.Do[AndWait]"
- Tentative `fyne.Do()` dans ShowSettings - pas suffisant
- À investiguer: interaction systray/Fyne thread model

**🔧 COMPILATION OBLIGATOIRE** (CGO/Fyne nécessite le bon GCC):
```bash
export PATH="/c/msys64/mingw64/bin:$PATH" && go build -o anemonesync.exe ./cmd/anemonesync/
```
- ❌ TDM-GCC 10.3.0 = binaire corrompu "n'est pas une application Win32 valide"
- ✅ MSYS2 MinGW64 GCC 15.2.0 = fonctionne

**Prochaines étapes**:
1. Corriger bug thread Fyne/systray (Settings window)
2. Tester remote watcher avec serveur SMB

## Session 018 - 2026-01-15
**Status**: ✅ Terminée (refactoring SMB complet)
**Durée**: ~1.5h
**Phase**: Refactoring SMB + Fix crash UI

**Réalisations**:
- **Fix crash Fyne/systray**: Refactorisé tray.go pour utiliser le driver desktop natif Fyne (plus de `fyne.io/systray`)
- **CLAUDE.md créé**: Documentation compilation obligatoire MSYS2 MinGW64 GCC
- **Refactoring SMB majeur**: Séparation serveur SMB / share
  - SMB Connection = serveur + credentials uniquement (pas de share)
  - Share sélectionné au niveau du job avec bouton "Refresh" pour lister les shares
  - Filtre automatique des shares administratifs ($)
- **ListSharesOnServer**: Nouvelle fonction pour énumérer les shares via go-smb2

**Fichiers modifiés** (10 fichiers, ~200 lignes modifiées):
- `internal/database/schema.sql` - Supprimé colonne share de smb_servers
- `internal/database/models.go` - Supprimé champ Share
- `internal/database/db.go` - Mis à jour queries CRUD
- `internal/smb/credentials.go` - Clé simplifiée (host uniquement)
- `internal/smb/client.go` - Ajout ListSharesOnServer()
- `internal/app/types.go` - SMBConnection sans Share
- `internal/app/app.go` - Mis à jour méthodes credentials
- `internal/app/smbform.go` - Formulaire sans champ share
- `internal/app/smblist.go` - Affichage simplifié
- `internal/app/jobform.go` - Ajout sélecteur de share avec refresh

**Build**: anemonesync.exe compile ✅, démarre ✅
**Note**: Base de données doit être recréée (nouveau schéma)

**Prochaines étapes**:
1. Tester workflow complet: ajouter serveur SMB → créer job → sélectionner share
2. Corriger icône systray (PNG corrompu)
3. Tester remote watcher avec serveur SMB réel

## Session 019 - 2026-01-18
**Status**: ✅ Terminée (nombreux bugfixes UI + DB)
**Durée**: ~1.5h
**Phase**: Bugfixes et améliorations UX

**Réalisations**:
- **Fix Fyne thread errors**: Enveloppé appels UI dans `fyne.Do()` (smbform.go, joblist.go)
- **Fix DB CHECK constraint**: `NULLIF(?, '')` pour domain/smb_version vides
- **Fix formulaire SMB**: Fermeture auto après save, stockage référence dialog
- **Browse remote path**: Nouveau composant `RemoteFolderBrowser` pour naviguer dans les shares SMB
- **Textes d'aide dynamiques**: Descriptions pour Sync Mode et Remote Watching dans le formulaire job
- **Fix édition job**:
  - Schedule lu depuis TriggerParams (pas TriggerMode)
  - SMBConnectionID trouvé par RemoteHost au chargement
  - Fix parseRemotePath (utilisait filepath.SplitList au lieu de splitPath)
  - Guard nil pour onSMBConnectionChanged
- **Fix sync request**: RemotePath utilise maintenant FullRemotePath() (UNC complet)
- **Fix DB GetSyncJob**: Utilise sql.NullInt64 pour last_run/next_run (comme GetAllSyncJobs)

**Fichiers modifiés** (8 fichiers, ~150 lignes):
- `internal/app/smbform.go` - fyne.Do(), dialog ref, fermeture auto
- `internal/app/jobform.go` - Browse remote, textes aide, guard nil
- `internal/app/joblist.go` - fyne.Do() dans Refresh()
- `internal/app/remote_browser.go` - Nouveau fichier (navigateur SMB)
- `internal/app/app.go` - ListRemoteFolders(), fix conversions DB↔App
- `internal/app/syncmanager.go` - FullRemotePath() dans SyncRequest
- `internal/database/db.go` - NULLIF, fix GetSyncJob scan types

**Build**: anemonesync.exe compile ✅
**Note**: Icône systray toujours corrompue (PNG invalide)

**Prochaines étapes**:
1. Corriger icône systray (PNG corrompu)
2. Tester synchronisation complète avec serveur SMB
3. Tests end-to-end du workflow complet

## Session 020 - 2026-01-18
**Status**: ✅ Terminée (synchronisation fonctionnelle)
**Durée**: ~1h
**Phase**: Bugfixes critiques synchronisation + icône

**Réalisations**:
- **Fix "Sync interval changed" log**: Settings utilise maintenant les valeurs actuelles de l'app (GetLogLevel/GetSyncInterval)
- **Fix "share cannot be empty"**: Parsing UNC path pour extraire server/share depuis RemotePath au lieu de ServerCredentialID
- **Fix "leading '\\' not allowed"**: Remote scanner utilise chemins relatifs au share (pas UNC complet)
- **Fix fichiers au mauvais endroit**: Normalisation des chemins (relatifs pour comparaison, absolus pour exécution)
- **Nouvelle icône**: anemone.png intégrée via go:embed (remplace placeholder corrompu)

**Fichiers modifiés** (4 fichiers, ~80 lignes):
- `internal/sync/engine.go` - parseUNCPath(), toRelativePath(), chemins normalisés
- `internal/app/settings.go` - Utilise valeurs actuelles pour Select widgets
- `internal/app/app.go` - Ajout GetLogLevel(), GetSyncInterval()
- `internal/app/icon.go` - go:embed assets/anemone.png

**Fichiers créés**:
- `internal/app/assets/anemone.png` - Icône anémone de mer

**Build**: anemonesync.exe compile ✅, synchronisation fonctionne ✅, icône OK ✅

**Prochaines étapes**:
1. Laisser tourner synchronisation complète
2. Vérifier intégrité des fichiers synchronisés
3. Tests de performance avec gros volumes

## Session 021 - 2026-01-18
**Status**: ✅ Terminée (Stop sync + Pause auto-sync)
**Durée**: ~1h
**Phase**: Améliorations UX synchronisation

**Réalisations**:
- **Bouton Stop Sync**: Arrêter une synchronisation en cours
  - Menu systray: "Stop Sync" (actif quand sync en cours)
  - Settings > Sync Jobs: Bouton "Stop" rouge
  - `SyncManager.CancelAllSyncs()`, `App.StopSync()`, `App.StopJobSync()`
- **Pause auto-sync**: Sync manuelle la première fois
  - Nouveau champ `PauseAutoSync` sur `SyncJob` (défaut: true pour nouveaux jobs)
  - Checkbox dans formulaire job: "Pause automatic sync (manual sync only)"
  - Scheduler, Watcher, RemoteWatcher respectent ce champ
- **Mise à jour CLAUDE.md**: Nouveau format avec Project Context, Session Management, Go docs standards

**Fichiers modifiés** (9 fichiers, ~200 lignes):
- `internal/app/syncmanager.go` - CancelAllSyncs(), GetRunningSyncJobIDs()
- `internal/app/app.go` - StopSync(), StopJobSync(), IsJobSyncing()
- `internal/app/tray.go` - Menu item "Stop Sync" dynamique
- `internal/app/settings.go` - Bouton Stop, updateSyncButtons()
- `internal/app/types.go` - PauseAutoSync field
- `internal/app/jobform.go` - Checkbox pause auto-sync
- `internal/app/watcher.go` - Respect PauseAutoSync
- `internal/app/scheduler.go` - Respect PauseAutoSync
- `internal/app/remote_watcher.go` - Respect PauseAutoSync
- `CLAUDE.md` - Nouveau format v2.0

**Build**: anemonesync.exe compile ✅

**Prochaines étapes**:
1. Intégration manifeste Anemone Server (accélérer scan remote)
2. Afficher taille sync dans liste des jobs
3. First sync wizard / guided setup

## Session 022 - 2026-01-18
**Status**: ✅ Terminée
**Durée**: ~30min
**Phase**: Mise à jour documentation selon nouveau CLAUDE.md

**Réalisations**:
- **ARCHITECTURE.md**: Ajout Phase 5-6, stats actualisées, roadmap mise à jour, version 1.0.0
- **README.md**: Statut v1.0 fonctionnel, instructions build MSYS2, structure projet
- **INSTALLATION.md**: MSYS2 MinGW64 obligatoire (pas TDM-GCC!), section utilisation
- **CONTRIBUTING.md**: Format commit `type(scope):`, build MSYS2, structure projet
- **model_CLAUDE.md**: Supprimé (template obsolète)

**Fichiers modifiés**: ARCHITECTURE.md, README.md, INSTALLATION.md, CONTRIBUTING.md
**Fichiers supprimés**: model_CLAUDE.md

**Prochaines étapes**:
1. Intégration manifeste Anemone Server (accélérer scan remote)
2. Afficher taille sync dans liste des jobs
3. First sync wizard / guided setup

## Session 023 - 2026-01-18
**Status**: ✅ Terminée
**Durée**: ~2h
**Phase**: Manifeste Anemone + Sync on Startup

**Réalisations**:
- **Manifeste Anemone**: Lecture `.anemone/manifest.json` pour scan remote ultra-rapide (~5ms vs minutes)
- **Sync on startup**: Option par job pour sync immédiate au démarrage Windows (flag `--autostart`)
- **Persistence options**: `PauseAutoSync`, `SyncOnStartup`, `RemoteWatch` stockés en JSON dans `network_conditions`
- **Fix race condition**: Délai 2s + flag `ready` pour éviter crash systray au démarrage
- **Fix logs**: "Sync already in progress" → DEBUG au lieu d'ERROR

**Fichiers créés**:
- `internal/sync/manifest.go` (~160 lignes) - Lecteur manifeste Anemone

**Fichiers modifiés** (12 fichiers):
- `internal/smb/client.go` - `ReadFile()`, fix déconnexion silencieuse
- `internal/sync/engine.go` - Intégration manifeste avec fallback SMB
- `internal/app/autostart.go` - Flag `--autostart` dans registre
- `internal/app/types.go` - `JobOptions` struct JSON, `SyncOnStartup`
- `internal/app/app.go` - `triggerStartupSync()`, conversions JSON, délai startup
- `internal/app/tray.go` - Flag `ready` anti-crash
- `internal/app/jobform.go` - Checkbox "Sync on startup"
- `internal/app/smbform.go` - Tip Anemone Server
- `internal/scanner/scanner.go` - Fix chemins relatifs dans cache
- `cmd/anemonesync/main.go` - Parse `--autostart`

## Session 024 - 2026-01-18
**Status**: ⚠️ Terminée avec BUG CRITIQUE non résolu
**Durée**: ~2h
**Phase**: Sync & Shutdown + Realtime Sync + Bugfix suppression fichiers

**Réalisations**:
- **Sync & Shutdown**: Fonctionnalité complète pour sync puis arrêter Windows
  - `shutdown.go` - ShutdownManager avec timeout, force option
  - `shutdown_dialog.go` - Dialog configuration (job, timeout, force)
  - `shutdown_progress.go` - Dialog progression avec annulation
  - Menu systray "Sync & Shutdown" avec sous-menu par job
  - Notifications "Shutdown Pending" / "Shutdown Cancelled"
  - Commande Windows `shutdown /s /t 30` (ou `/f` pour forcer)

- **Realtime Sync**: Sync immédiat sur changements fichiers locaux
  - Nouveaux champs `RealtimeSync` et `RealtimeSyncDelay` sur SyncJob
  - Checkbox + sélecteur délai (1s/3s/5s/10s) dans formulaire job
  - Watcher utilise le délai spécifique au job

- **Tentative fix suppression fichiers** (ÉCHEC):
  - Modifié `detector.go` pour détecter fichiers "recréés" (même contenu, temps plus récent)
  - Ajouté `isFileRecreated()` pour distinguer recréation vs modification
  - Tests passent mais **le bug persiste en production**

**Fichiers créés** (4):
- `internal/app/shutdown.go` (~270 lignes)
- `internal/app/shutdown_dialog.go` (~150 lignes)
- `internal/app/shutdown_progress.go` (~130 lignes)
- `cmd/anemonesync/icon.ico` (multi-résolution)

**Fichiers modifiés** (9):
- `internal/cache/detector.go` - isFileRecreated(), fix Case 3 & 4
- `internal/app/types.go` - RealtimeSync, RealtimeSyncDelay
- `internal/app/jobform.go` - UI realtime sync
- `internal/app/watcher.go` - parseRealtimeDelay(), RealtimeSync check
- `internal/app/tray.go` - Menu Sync & Shutdown
- `internal/app/app.go` - ShutdownManager integration
- `internal/app/notifications.go` - ShutdownPending/Cancelled
- `internal/app/syncmanager.go` - ExecuteSyncAndWait()

**🚨 BUG CRITIQUE NON RÉSOLU**:
- **Symptôme**: Fichiers ajoutés localement sont SUPPRIMÉS au lieu d'être uploadés
- **Cause probable**: Le change detector reçoit des données incorrectes (cache/remote/local)
- **Investigation requise**: Tracer exactement ce que reçoit `DetermineSyncAction()`
- **Workaround**: Désactiver sync automatique, utiliser sync manuelle uniquement

**Icône .exe**: Reportée (problème windres preprocessing)

**Prochaines étapes CRITIQUES**:
1. **URGENT**: Débugger le flux complet de détection de changements
2. Tracer: localFiles, remoteFiles, cachedFiles avant BatchDetermineSyncActions
3. Vérifier que les chemins sont cohérents (relatifs partout)
4. Icône .exe via `fyne package` ou `rsrc`

## Session 025 - 2026-01-18
**Status**: ✅ Terminée (bug critique résolu)
**Durée**: ~1h
**Phase**: Bugfix critique - Fichiers supprimés au lieu d'être uploadés

**Problème résolu**: Nouveaux fichiers locaux étaient supprimés au lieu d'être uploadés vers le serveur.

**Causes identifiées (3 bugs)**:
1. **scanner.go**: `foundFiles` utilisait des chemins absolus, `detectDeletedFiles` comparait avec des chemins relatifs
2. **db.go**: Entrées corrompues avec chemins absolus dans `files_state`
3. **cache.go**: Fichiers jamais sync (last_sync=NULL) étaient traités comme "dans le cache"

**Corrections**:
- `scanner.go`: Utiliser chemins relatifs pour `foundFiles`
- `db.go`: Ajout `cleanupCorruptedCacheEntries()` au démarrage
- `cache.go`: `GetCachedState`/`GetAllCachedFiles` retournent nil si last_sync=NULL

**Fichiers modifiés**: scanner.go, db.go, cache.go
**Commit**: 542cc2f "fix(sync): Fix new local files being deleted instead of uploaded"

**Note**: Le manifeste Anemone Server peut causer des re-uploads si pas mis à jour rapidement après upload. À surveiller.

**Prochaines étapes**:
1. Surveiller comportement avec manifeste Anemone
2. Ajouter fallback SMB si fichier pas dans manifeste mais dans cache
3. Icône .exe

---

## Instructions de maintenance

- Chaque session doit avoir un résumé de 3-5 lignes maximum
- Format: Session XXX - Date, Status, Objectif, Réalisations (bullet points courts), Prochaines étapes
- Quand une session est terminée, mettre Status à "Terminée"
- Référencer le(s) fichier(s) session détaillé(s)
