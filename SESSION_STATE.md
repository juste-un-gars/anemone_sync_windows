# AnemoneSync - Session State

Ce fichier contient un index court de chaque session. Les details sont dans `sessions/session_XXX.md`.

**Derniere session**: 055 (2026-01-28)
**Phase en cours**: Cloud Files API - Folder accessible offline, hydration a tester
**Prochaine session**: 056 - Test hydration + finalisation Cloud Files

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
**Resume**: SESSION_STATE.md 1217->187 lignes, ajout "File Size Guidelines" dans CLAUDE.md

## Session 043 - 2026-01-25
**Status**: Done | **Phase**: Refactoring db.go
**Resume**: db.go 1058->5 fichiers (db.go, db_files.go, db_jobs.go, db_servers.go, db_config.go)

## Session 044 - 2026-01-25
**Status**: Done | **Phase**: Bugfixes tests
**Resume**: Fix UpsertFileState (last_sync), fix SMB tests (ancienne API), fix Scanner tests (SimulateSyncComplete)

## Session 045 - 2026-01-25
**Status**: Done | **Phase**: Refactoring engine.go + app.go
**Resume**: engine.go 1007->4 fichiers, app.go 1512->6 fichiers, tous < 500 lignes

## Session 046 - 2026-01-25
**Status**: Done | **Phase**: Refactoring fichiers > 500 lignes
**Resume**: 5 fichiers refactores (syncmanager, jobform, smb/client, cloudfiles/provider, cloudfiles/dehydration)

## Session 047 - 2026-01-25
**Status**: Done | **Phase**: Refactoring cfapi.go
**Resume**: cfapi.go 500->4 fichiers (cfapi.go 128, cfapi_syncroot.go 121, cfapi_placeholder.go 115, cfapi_operations.go 175)

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

## Session 052 - 2026-01-27
**Status**: Partial | **Phase**: Debug Cloud Files - Approche CloudMirror
**Resume**: Decouverte sample Microsoft, FileIdentity fix, placeholders crees OK mais dossier inaccessible

## Session 053 - 2026-01-27
**Status**: Done | **Phase**: Debug Cloud Files - Logs detailles
**Resume**: Ajout logs debug complets dans bridge C, fix FETCH_PLACEHOLDERS incompatible avec ALWAYS_FULL

## Session 054 - 2026-01-27
**Status**: Done | **Phase**: Debug Cloud Files - Fix navigation
**Resume**: Fix debounce callback, navigation OK avec app running

## Session 055 - 2026-01-28
**Status**: Partial | **Phase**: Cloud Files - Folder accessible offline
**Resume**: Dossier accessible meme sans provider, fix path hydration, hydration a tester

---

## Decouvertes Session 055

### PROBLEME RESOLU: Dossier inaccessible quand app fermee
**Cause**: Le dossier sync root etait lui-meme traite comme un placeholder.
Windows demandait au provider de lister son contenu = erreur si app fermee.

**Solution** (basee sur recherche web):
1. **Dossiers = vrais dossiers NTFS** (pas placeholders) via `os.MkdirAll`
2. **Fichiers = placeholders** (seuls eux necessitent le provider)
3. **CF_POPULATION_POLICY_ALWAYS_FULL** - provider pre-cree tout
4. **CF_REGISTER_FLAG_DISABLE_ON_DEMAND_POPULATION_ON_ROOT** - pas de callback pour root
5. **CF_REGISTER_FLAG_MARK_IN_SYNC_ON_ROOT** - root marque in-sync
6. **FETCH_PLACEHOLDERS non enregistre** - coherent avec ALWAYS_FULL

### Comportement actuel (comme OneDrive):
- Dossier sync root accessible meme sans l'app
- Fichiers visibles avec icone cloud
- Ouvrir fichier cloud-only sans app = erreur "provider not running" (normal)
- Ouvrir fichier cloud-only avec app = devrait telecharger (hydration)

### Bug trouve: Path hydration incorrect
**Symptome**: `test_anemone/test_anemone/L_20260127...` au lieu de `test_anemone/PXL_20260127...`

**Cause**: NormalizedPath de Windows = `\test_anemone\PXL...` (sans lettre lecteur)
On essayait de retirer `D:\test_anemone` ce qui tronquait mal.

**Fix**: Nouveau parsing dans `hydration.go`:
1. Strip leading `\`
2. Strip sync root folder name (`test_anemone\`)
3. Resultat = chemin relatif correct (`PXL_20260127_091021514.jpg`)

### Fichiers modifies Session 055:
- `internal/cloudfiles/types.go` - ALWAYS_FULL policy
- `internal/cloudfiles/cfapi_bridge.c` - FETCH_PLACEHOLDERS retire
- `internal/cloudfiles/sync_root.go` - Flags DISABLE_ON_DEMAND + MARK_IN_SYNC
- `internal/cloudfiles/placeholder_manager.go` - Dossiers = vrais NTFS (os.MkdirAll)
- `internal/cloudfiles/hydration.go` - Fix parsing NormalizedPath

## Prochaines etapes (Session 056)

### A tester:
1. **Hydration**: Ouvrir fichier placeholder avec app running
2. **Dehydration**: Liberer espace sur fichier telecharge
3. **Sous-dossiers**: Navigation dans arborescence profonde

### Si hydration fonctionne:
1. Nettoyer logs debug
2. Tester avec gros fichiers
3. Integration finale

**Sources utiles:**
- https://learn.microsoft.com/en-us/windows/win32/api/cfapi/ne-cfapi-cf_register_flags
- https://learn.microsoft.com/en-us/answers/questions/2288103/cloud-file-api-faq
