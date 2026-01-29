# AnemoneSync - Session State

Ce fichier contient un index court de chaque session. Les details sont dans `sessions/session_XXX.md`.

**Derniere session**: 064 (2026-01-29)
**Phase en cours**: Cloud Files API Complete
**Prochaine etape**: CLI dehydrate, tests gros fichiers

---

## Session 001 - 2026-01-11
**Status**: Done | **Phase**: Phase 0 + Infrastructure
**Resume**: 35 fichiers, CI/CD, docs, Makefile

## Session 002 - 2026-01-11
**Status**: Done | **Phase**: Phase 1 Scanner
**Resume**: 7 modules scanner, 65+ tests

## Session 003 - 2026-01-11
**Status**: Done | **Phase**: Phase 1 Scanner (fixes)
**Resume**: 97% tests, fix deadlock/DB

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
**Resume**: db.go 1058->5 fichiers

## Session 044 - 2026-01-25
**Status**: Done | **Phase**: Bugfixes tests
**Resume**: Fix UpsertFileState, SMB tests, Scanner tests

## Session 045 - 2026-01-25
**Status**: Done | **Phase**: Refactoring engine.go + app.go
**Resume**: engine.go 1007->4 fichiers, app.go 1512->6 fichiers

## Session 046 - 2026-01-25
**Status**: Done | **Phase**: Refactoring fichiers > 500 lignes
**Resume**: 5 fichiers refactores

## Session 047 - 2026-01-25
**Status**: Done | **Phase**: Refactoring cfapi.go
**Resume**: cfapi.go 500->4 fichiers

## Session 048 - 2026-01-27
**Status**: Done | **Phase**: CLI Interface
**Resume**: CLI complet (--help, --list-jobs, --sync, --sync-all)

## Session 049 - 2026-01-27
**Status**: Partial | **Phase**: Debug Cloud Files API
**Resume**: Probleme identifie = callbacks Go incompatibles avec threading Windows

## Session 050 - 2026-01-27
**Status**: Done | **Phase**: CGO Bridge Cloud Files
**Resume**: Wrapper CGO complet (cfapi_bridge.c/h/go), 55 tests

## Session 051 - 2026-01-27
**Status**: Partial | **Phase**: Test CGO Bridge
**Resume**: FETCH_PLACEHOLDERS callback + debounce, dossier accessible

## Session 052 - 2026-01-27
**Status**: Partial | **Phase**: Debug Cloud Files - Approche CloudMirror
**Resume**: Decouverte sample Microsoft, FileIdentity fix

## Session 053 - 2026-01-27
**Status**: Done | **Phase**: Debug Cloud Files - Logs detailles
**Resume**: Fix FETCH_PLACEHOLDERS incompatible avec ALWAYS_FULL

## Session 054 - 2026-01-27
**Status**: Done | **Phase**: Debug Cloud Files - Fix navigation
**Resume**: Fix debounce callback, navigation OK

## Session 055 - 2026-01-28
**Status**: Done | **Phase**: Cloud Files - Folder accessible offline
**Resume**: Dossier accessible sans provider, fix path hydration

## Session 056 - 2026-01-28
**Status**: Partial | **Phase**: Cloud Files - Fix hydration bugs
**Resume**: 4 bugs corriges, hydration echoue encore

## Session 057 - 2026-01-28
**Status**: Partial | **Phase**: Cloud Files - Architecture synchrone bloquante
**Resume**: Decouverte cause racine, nouvelle architecture avec Event Windows

## Session 058 - 2026-01-28
**Status**: Partial | **Phase**: Cloud Files - Fix crash GC Go
**Resume**: Crash corrige (unsafe.Pointer -> uintptr pour HANDLE)

## Session 059 - 2026-01-28
**Status**: Done | **Phase**: Cloud Files - Fix structure C incorrecte
**Resume**: CfExecute immediat fonctionne! Structure CF_OPERATION_TRANSFER_DATA_PARAMS corrigee

## Session 060 - 2026-01-28
**Status**: Partial | **Phase**: Cloud Files - Restauration vraie logique hydration
**Resume**: Supprime code de test, restaure enqueue vers Go. A tester apres reboot.

## Session 061 - 2026-01-28
**Status**: Done | **Phase**: Cloud Files - Fix MARK_IN_SYNC flag
**Resume**: Ajout flag MARK_IN_SYNC sur dernier chunk TransferData

## Session 062 - 2026-01-28
**Status**: Done | **Phase**: Cloud Files - HYDRATION COMPLETE!
**Resume**: Fix VALIDATE_DATA callback - specifier Offset/Length pour ACK_DATA

**Probleme identifie:**
- TransferData SUCCESS avec MARK_IN_SYNC, mais fichier reste placeholder
- Windows envoyait CANCEL_FETCH_DATA apres 1 minute de timeout
- Le callback VALIDATE_DATA ne specifiait pas la plage de donnees validees

**Cause racine:**
Dans `OnValidateDataCallback`, l'appel a `ACK_DATA` ne specifiait pas:
- `opInfo.RequestKey`
- `opParams.AckData.Offset` (devait etre 0)
- `opParams.AckData.Length` (devait etre FileSize)

**Correction appliquee (cfapi_bridge.c):**
```c
opInfo.RequestKey = callbackInfo->RequestKey;
opParams.AckData.Offset.QuadPart = 0;
opParams.AckData.Length.QuadPart = callbackInfo->FileSize;
```

**Resultat:**
- Hydration fonctionne! Double-clic sur placeholder -> fichier telecharge et s'ouvre
- Attribut fichier passe de "A O" (Archive+Offline) a "A" (Archive seul)
- Plus de CANCEL_FETCH_DATA timeout

## Session 063 - 2026-01-29
**Status**: Done | **Phase**: Dehydration UI
**Resume**: Menu Free Up Space, dialog fichiers hydrates, scanner

## Session 064 - 2026-01-29
**Status**: Done | **Phase**: Dehydration Complete
**Resume**: Fix dehydration via CfUpdatePlaceholder avec DEHYDRATE|MARK_IN_SYNC

**Probleme initial:**
- `CfSetInSyncState` et `CfDehydratePlaceholder` retournaient SUCCESS mais ne changeaient pas le state
- Le fichier restait state=0x21 (PLACEHOLDER+SYNC_ROOT) sans PARTIAL ni IN_SYNC

**Solution trouvee:**
Utiliser `CfUpdatePlaceholder` avec les flags combines:
```go
CF_UPDATE_FLAG_DEHYDRATE | CF_UPDATE_FLAG_MARK_IN_SYNC  // 0x6
```

**Resultat:**
```
Before: state=0x00000021 (PLACEHOLDER + SYNC_ROOT)
After:  state=0x00000031 (PLACEHOLDER + PARTIAL + SYNC_ROOT)
```
- Le flag PARTIAL (0x10) indique fichier deshydrate
- L'espace disque est reellement libere ("Taille sur le disque" = 0)
- Attributs Explorer: ALOM (placeholder) <-> AL (hydrate) <-> ALOM (deshydrate)

**Fichiers modifies:**
- `internal/cloudfiles/cfapi_operations.go` - Ajout UpdatePlaceholder() avec tous les CF_UPDATE_FLAG_*
- `internal/cloudfiles/hydration.go` - Utilise UpdatePlaceholder pour marquer IN_SYNC apres hydration
- `internal/cloudfiles/dehydration_scan.go` - Utilise UpdatePlaceholder(DEHYDRATE|MARK_IN_SYNC)

**Sources cles:**
- https://learn.microsoft.com/en-us/windows/win32/api/cfapi/nf-cfapi-cfupdateplaceholder
- https://learn.microsoft.com/en-us/answers/questions/2288103/cloud-file-api-faq

---

## Prochaines etapes

### Session 065 - Tests automatises Cloud Files

**Objectif:** Creer un outil CLI de test automatise pour valider hydratation/dehydratation

**Environnement de test:**
- Local sync root: `D:\test_anemone\_test\` (sous-dossier dedie)
- Remote: `\\192.168.83.221\data_franck\test_anemone\_test\`
- Fichiers sources: `D:\temp\` (variete de fichiers disponibles)

**Fichiers sources disponibles dans D:\temp:**
- Petits: bbox ipv6.txt (190B), dhcp.txt (955B)
- Moyens: .docx (18-487KB), .xlsx (9-25KB)
- Gros: Ferdium exe (345MB), linuxmint ISO (3GB)
- Structure: 1/A/B/*.docx (3 niveaux imbriques)

**Outil a creer:** `cmd/cloudfiles_test/`
```
main.go       - CLI (--list, --run-all, --run T1,T2)
config.go     - Chemins, credentials SMB
scenarios.go  - Definition des scenarios
runner.go     - Execution des tests
helpers.go    - Copie fichiers, verif etats
report.go     - Affichage PASS/FAIL
```

**Scenarios a implementer:**
| ID | Scenario | Description |
|----|----------|-------------|
| T1 | Hydratation basique | Fichier distant -> placeholder -> lire -> hydrate |
| T2 | Dehydratation basique | Fichier hydrate -> DehydrateFile() -> PARTIAL |
| T3 | Cycle complet | Hydrate -> Modify -> Sync -> Dehydrate |
| T4 | Upload local | Creer fichier local -> Sync -> Upload serveur |
| T5 | Structure imbriquee | Dossier 1/A/B/ avec sous-fichiers |
| T6 | Gros fichier | 345MB exe, verifier progress |
| T7 | Modif serveur | Modifier fichier distant -> Sync -> MAJ placeholder |
| T8 | Suppression serveur | Supprimer distant -> Sync -> Suppression locale |

**Options:**
- `--list` : Lister les scenarios disponibles
- `--run-all` : Executer tous les tests
- `--run T1,T2,T3` : Executer tests specifiques
- `--cleanup` : Nettoyer apres (optionnel, garde par defaut)

---

### Autres taches futures

1. **CLI**: `anemonesync --dehydrate <job-id> [--days 30]`
2. **Cleanup**: Supprimer les logs DEBUG excessifs
3. **Tests**: Serveur offline, interruption reseau
