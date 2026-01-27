# AnemoneSync - Session State

Ce fichier contient un index court de chaque session. Les details sont dans `sessions/session_XXX.md`.

**Derniere session**: 054 (2026-01-27)
**Phase en cours**: Cloud Files API - Navigation OK, creation fichiers a tester
**Prochaine session**: 055 - Test creation fichiers + finalisation Cloud Files

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

## Session 052 - 2026-01-27
**Status**: Partial | **Phase**: Debug Cloud Files - Approche CloudMirror
**Resume**: Decouverte sample Microsoft, FileIdentity fix, placeholders crees OK mais dossier inaccessible

## Session 053 - 2026-01-27
**Status**: Done | **Phase**: Debug Cloud Files - Logs detailles
**Resume**: Ajout logs debug complets dans bridge C, fix FETCH_PLACEHOLDERS incompatible avec ALWAYS_FULL

## Session 054 - 2026-01-27
**Status**: Partial | **Phase**: Debug Cloud Files - Fix navigation
**Resume**: Fix debounce callback, navigation OK, creation fichiers boucle infinie a resoudre

---

## Bugs connus

- **Cloud Files API**: Creation fichiers dans sync root - a tester avec config ALWAYS_FULL sans FETCH_PLACEHOLDERS

## Decouvertes Session 054

### PROBLEME RESOLU: Dossier inaccessible
**Cause**: Le debounce dans `OnFetchPlaceholdersCallback` faisait `return` sans repondre au callback.
Windows attendait une reponse qui ne venait jamais = freeze.

**Solution**: Toujours repondre aux callbacks, meme si debounce.

### Ce qui fonctionne maintenant:
- Navigation dans le dossier sync root (entrer/sortir multiple fois)
- Callbacks FETCH_PLACEHOLDERS recus et traites correctement
- Logs debug complets dans cfapi_bridge.c

### Nouveau probleme: Creation fichiers
- Avec `CF_POPULATION_POLICY_PARTIAL` + `FETCH_PLACEHOLDERS`: boucle infinie de callbacks
- Windows rappelle FETCH_PLACEHOLDERS en continu quand on cree un fichier

### Configuration actuelle (a tester):
- `CF_POPULATION_POLICY_ALWAYS_FULL` (provider gere tout)
- `FETCH_PLACEHOLDERS` **non enregistre** (coherent avec ALWAYS_FULL)
- Tous les autres callbacks enregistres (FETCH_DATA, VALIDATE_DATA, NOTIFY_*)

### Fichiers modifies Session 054:
- `internal/cloudfiles/cfapi_bridge.c` - Fix debounce, retire FETCH_PLACEHOLDERS
- `internal/cloudfiles/types.go` - Retour a ALWAYS_FULL

## Decouvertes Session 052

### Ce qui fonctionne maintenant:
- Placeholders crees avec succes (`HRESULT=0x00000000, processed=1/1`)
- FileIdentity obligatoire corrige (utilise path comme identity)
- Policy PLACEHOLDER_MANAGEMENT = UNRESTRICTED
- Flag CF_REGISTER_FLAG_UPDATE pour forcer mise a jour policies
- Population placeholders automatique au demarrage

### Ce qui ne fonctionne PAS:
- Acces au dossier sync root bloque meme avec app en cours d'execution
- Erreur "Le fournisseur de fichier cloud n'est pas en cours d'execution" quand app fermee

### Decouvertes importantes (recherche web):
1. **Sample CloudMirror Microsoft n'implemente PAS FETCH_PLACEHOLDERS**
   - Ils creent les placeholders a l'avance avec CfCreatePlaceholders
   - Seulement FETCH_DATA et CANCEL_FETCH_DATA sont enregistres

2. **FileIdentity est OBLIGATOIRE pour les fichiers**
   - Documentation: "FileIdentity is required for files (not for directories)"
   - On utilisait le path comme identity (comme CloudMirror)

3. **CF_HYDRATION_POLICY_ALWAYS_FULL interdit CfCreatePlaceholders**
   - Notre code utilise FULL (2) pas ALWAYS_FULL (3) donc OK

4. **CFAPI ne gere PAS la creation de fichiers locaux**
   - Il faut ReadDirectoryChangesW ou USN Journal pour detecter
   - Pas de callback automatique pour nouveaux fichiers

## Decouvertes Session 053

### Comportement observe:
- **Dossier accessible UNE SEULE FOIS** apres lancement app
- Si on sort et re-entre, dossier bloque (meme avec app en cours)
- Apres redemarrage app: fonctionne encore une fois puis bloque

### Probleme identifie:
- FETCH_PLACEHOLDERS etait enregistre mais on utilise CF_POPULATION_POLICY_ALWAYS_FULL
- C'est **INCOMPATIBLE**: ALWAYS_FULL = provider gere tout, pas de callback FETCH_PLACEHOLDERS
- Quand on repondait avec DISABLE_ON_DEMAND_POPULATION, Windows desactivait les callbacks

### Corrections Session 053:
1. **FETCH_PLACEHOLDERS retire** des callbacks enregistres (incompatible avec ALWAYS_FULL)
2. **Erreur ALREADY_EXISTS ignoree** (0x800700B7 = fichier existe deja, pas une erreur)
3. **Logs debug complets** ajoutes dans cfapi_bridge.c avec timestamp

### Fichiers modifies Session 053:
- `internal/cloudfiles/cfapi_bridge.c` - Logs detailles + FETCH_PLACEHOLDERS retire
- `internal/cloudfiles/cfapi_placeholder.go` - Ignore erreur ALREADY_EXISTS

## Prochaines etapes (Session 055)

### A tester:
1. **Navigation**: devrait fonctionner (entrer/sortir du dossier)
2. **Creation fichiers**: avec ALWAYS_FULL + sans FETCH_PLACEHOLDERS
3. **Hydration**: ouvrir un fichier placeholder (download depuis serveur)

### Si creation fichiers ne fonctionne pas:
1. Verifier les logs - quel callback bloque?
2. Essayer de desactiver certains NOTIFY_* callbacks
3. Comparer avec CloudMirror qui utilise seulement FETCH_DATA + CANCEL_FETCH_DATA

### Si ca fonctionne:
1. Nettoyer les callbacks inutiles (garder minimum necessaire)
2. Tester hydration complete (download fichier)
3. Tester dehydration (liberer espace)
4. Integration finale avec sync engine

**Alternative si Cloud Files reste instable:**
- Garder sync traditionnelle comme fonctionnalite principale
- Cloud Files en option "beta" ou desactivee par defaut

**Sources utiles:**
- https://github.com/microsoft/Windows-classic-samples/tree/main/Samples/CloudMirror
- https://learn.microsoft.com/en-us/answers/questions/2288103/cloud-file-api-faq
- https://learn.microsoft.com/en-us/windows/win32/api/cfapi/ns-cfapi-cf_placeholder_create_info
