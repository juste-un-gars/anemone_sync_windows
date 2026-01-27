# AnemoneSync - Session State

Ce fichier contient un index court de chaque session. Les details sont dans `sessions/session_XXX.md`.

**Derniere session**: 051 (2026-01-27)
**Phase en cours**: Cloud Files API - FETCH_PLACEHOLDERS partiellement resolu
**Prochaine session**: 052 - Resoudre creation fichiers dans sync root

---

## Session 001 - 2026-01-11
**Status**: Done | **Phase**: Phase 0 + Infrastructure
**Resume**: 35 fichiers, CI/CD, docs, Makefile | **Details**: sessions/session_001.md

## Session 002 - 2026-01-11
**Status**: Done | **Phase**: Phase 1 Scanner
**Resume**: 7 modules scanner, 65+ tests | **Details**: sessions/session_002.md

## Session 003 - 2026-01-11
**Status**: Done | **Phase**: Phase 1 Scanner (fixes)
**Resume**: 97% tests, fix deadlock/DB | **Details**: sessions/session_003.md

## Session 004 - 2026-01-11
**Status**: Done | **Phase**: Phase 1 Complete
**Resume**: 100% tests (63/63) | **Commit**: dad73a1

## Session 005 - 2026-01-11
**Status**: Done | **Phase**: Phase 2 SMB (Paliers 1-2)
**Resume**: Connect/Download/Upload | **Commit**: 9c5b175

## Session 006 - 2026-01-12
**Status**: Done | **Phase**: Phase 2 SMB + Phase 3 Cache
**Resume**: Keyring auth, 3-way merge | **Commits**: 2aaf5ae, d487d0e, e4550cd

## Session 007 - 2026-01-13
**Status**: Done | **Phase**: Phase 4 Sync Engine (Palier 1)
**Resume**: Engine, Executor, Errors | **Commit**: a13353b

## Session 008 - 2026-01-13
**Status**: Done | **Phase**: Phase 4 (Palier 2)
**Resume**: RemoteScanner, ProgressTracker | **Commit**: 22de4af

## Session 009 - 2026-01-13
**Status**: Done | **Phase**: Phase 4 (Palier 3)
**Resume**: Retry, ConflictResolver | **Commit**: fea5e1e

## Session 010 - 2026-01-13
**Status**: Done | **Phase**: Phase 4 Complete
**Resume**: WorkerPool, 71+ tests | **Commit**: cf3da27

## Session 011 - 2026-01-14
**Status**: Done | **Phase**: Documentation
**Resume**: Sessions 004-010 + ARCHITECTURE.md

## Session 012 - 2026-01-14
**Status**: Done | **Phase**: Phase 5 Desktop (Palier 1)
**Resume**: Fyne + System Tray

## Session 013 - 2026-01-14
**Status**: Done | **Phase**: Fix CGO
**Resume**: MSYS2 MinGW64 GCC requis (pas TDM-GCC)

## Session 014 - 2026-01-14
**Status**: Done | **Phase**: Phase 5 (Palier 2)
**Resume**: Settings UI, job form, status colors

## Session 015 - 2026-01-14
**Status**: Done | **Phase**: Phase 5 (Palier 3)
**Resume**: DB persistence, auto-start, notifications

## Session 016 - 2026-01-14
**Status**: Done | **Phase**: Phase 5 Complete
**Resume**: Scheduler, file watchers, sync integration

## Session 017 - 2026-01-15
**Status**: Partial | **Phase**: Phase 6 Remote Watcher
**Resume**: Polling SMB, bug thread Fyne non resolu

## Session 018 - 2026-01-15
**Status**: Done | **Phase**: Refactoring SMB
**Resume**: Fix crash Fyne, separation serveur/share, CLAUDE.md cree

## Session 019 - 2026-01-18
**Status**: Done | **Phase**: Bugfixes UI/DB
**Resume**: fyne.Do(), browse remote, edit job fixes

## Session 020 - 2026-01-18
**Status**: Done | **Phase**: Sync fonctionnelle
**Resume**: Fix chemins, nouvelle icone anemone.png

## Session 021 - 2026-01-18
**Status**: Done | **Phase**: UX Sync
**Resume**: Bouton Stop, Pause auto-sync

## Session 022 - 2026-01-18
**Status**: Done | **Phase**: Documentation
**Resume**: Mise a jour ARCHITECTURE/README/INSTALL/CONTRIBUTING

## Session 023 - 2026-01-18
**Status**: Done | **Phase**: Manifeste Anemone
**Resume**: Scan remote ultra-rapide, sync on startup

## Session 024 - 2026-01-18
**Status**: Partial | **Phase**: Sync & Shutdown
**Resume**: Shutdown Windows, realtime sync, BUG suppression fichiers

## Session 025 - 2026-01-18
**Status**: Done | **Phase**: Bugfix critique
**Resume**: Fix fichiers supprimes au lieu d'upload | **Commit**: 542cc2f

## Session 026 - 2026-01-18
**Status**: Done | **Phase**: Icone .exe
**Resume**: anemone.ico via fyne package

## Session 027 - 2026-01-18
**Status**: Done | **Phase**: Fallback SMB + taille sync
**Resume**: Manifeste fallback, affichage taille jobs

## Session 028 - 2026-01-18
**Status**: Done | **Phase**: Phase 7 (Palier 1)
**Resume**: Bindings Go cfapi.dll, 7 tests

## Session 029 - 2026-01-18
**Status**: Done | **Phase**: Phase 7 (Palier 2)
**Resume**: SyncRoot, Placeholders, Hydration, 32 tests

## Session 030 - 2026-01-18
**Status**: Done | **Phase**: Phase 7 (Palier 3)
**Resume**: Hydration a la demande, progress, cancel

## Session 031 - 2026-01-18
**Status**: Done | **Phase**: Phase 7 (Palier 4)
**Resume**: DehydrationManager, liberer espace auto

## Session 032 - 2026-01-18
**Status**: Done | **Phase**: Phase 7 (Palier 5)
**Resume**: UI Files On Demand, persistence JSON

## Session 033 - 2026-01-18
**Status**: Done | **Phase**: Phase 7 (Palier 6)
**Resume**: Integration SyncEngine, placeholders au lieu de download

## Session 034 - 2026-01-18
**Status**: Done | **Phase**: UI Simplification
**Resume**: TriggerMode unique (manual/5m/15m/30m/1h/realtime)

## Session 035 - 2026-01-18
**Status**: Partial | **Phase**: Debug Files On Demand
**Resume**: ERROR_CLOUD_FILE_NOT_UNDER_SYNC_ROOT non resolu

## Session 036 - 2026-01-19
**Status**: Done | **Phase**: First Sync Wizard
**Resume**: Analyse local vs remote, choix mode merge/wins

## Session 037 - 2026-01-19
**Status**: Done | **Phase**: Bugfix critique
**Resume**: Scanner ne met plus a jour cache pendant scan

## Session 038 - 2026-01-19
**Status**: Done | **Phase**: Bugfixes + Wizard
**Resume**: 5 bugs fixes, choix conflit keep_both, cooldown watcher

## Session 039 - 2026-01-20
**Status**: Done | **Phase**: Test Harness
**Resume**: Framework tests auto, 29 scenarios

## Session 040 - 2026-01-20
**Status**: Done | **Phase**: Test Harness v2
**Resume**: Utilise vrai sync.Engine, 25/29 tests

## Session 041 - 2026-01-20
**Status**: Done | **Phase**: Test Harness Complete
**Resume**: 29/29 tests passent, fix modes/conflits

## Session 042 - 2026-01-25
**Status**: Done | **Phase**: Cleanup + Guidelines
**Resume**: SESSION_STATE.md 1217→187 lignes, ajout "File Size Guidelines" dans CLAUDE.md

## Session 043 - 2026-01-25
**Status**: Done | **Phase**: Refactoring db.go
**Resume**: db.go 1058→5 fichiers (db.go, db_files.go, db_jobs.go, db_servers.go, db_config.go)

## Session 044 - 2026-01-25
**Status**: Done | **Phase**: Bugfixes tests
**Resume**: Fix UpsertFileState (last_sync), fix SMB tests (ancienne API), fix Scanner tests (SimulateSyncComplete)

## Session 045 - 2026-01-25
**Status**: Done | **Phase**: Refactoring engine.go + app.go
**Resume**: engine.go 1007→4 fichiers, app.go 1512→6 fichiers, tous < 500 lignes

## Session 046 - 2026-01-25
**Status**: Done | **Phase**: Refactoring fichiers > 500 lignes
**Resume**: 5 fichiers refactores (syncmanager, jobform, smb/client, cloudfiles/provider, cloudfiles/dehydration)

## Session 047 - 2026-01-25
**Status**: Done | **Phase**: Refactoring cfapi.go
**Resume**: cfapi.go 500→4 fichiers (cfapi.go 128, cfapi_syncroot.go 121, cfapi_placeholder.go 115, cfapi_operations.go 175)

## Session 048 - 2026-01-27
**Status**: Done | **Phase**: CLI Interface
**Resume**: CLI complet (--help, --list-jobs, --sync, --sync-all), docs mises a jour

## Session 049 - 2026-01-27
**Status**: Partial | **Phase**: Debug Cloud Files API
**Resume**: Investigation approfondie, probleme identifie = callbacks Go incompatibles avec threading Windows

## Session 050 - 2026-01-27
**Status**: Done | **Phase**: CGO Bridge Cloud Files
**Resume**: Wrapper CGO complet (cfapi_bridge.c/h/go), evite Go scheduler issues, compilation OK, 55 tests

## Session 051 - 2026-01-27
**Status**: Partial | **Phase**: Test CGO Bridge
**Resume**: Active UseCGOBridge, FETCH_PLACEHOLDERS callback + debounce, dossier accessible mais creation fichier bloque

---

## Bugs connus

- **Cloud Files API**: Dossier sync root accessible mais creation de fichiers bloque (Session 051)
  - FETCH_PLACEHOLDERS callback implemente avec debounce
  - CfExecute TRANSFER_PLACEHOLDERS retourne E_INVALIDARG (structure incorrecte)
  - Workaround: retour sans CfExecute + debounce 1s
  - A investiguer: callbacks NOTIFY_* pour creation/modification fichiers

## Prochaines etapes

### Session 052 - Creation fichiers dans sync root

**Probleme actuel:**
- Dossier s'ouvre correctement
- Creation de fichier bloque (operation cloud n'a pas termine)
- Peut necessiter callback supplementaire (VALIDATE_DATA? NOTIFY_FILE_OPEN_COMPLETION?)

**Pistes:**
1. Ajouter logs debug pour voir quel callback est appele lors de creation fichier
2. Implementer callbacks manquants (VALIDATE_DATA, FILE_OPEN_COMPLETION, etc.)
3. Verifier si le probleme vient de la policy d'hydratation (FULL vs PROGRESSIVE)
4. Alternative: desactiver Cloud Files API si trop complexe et garder sync traditionnelle
