# Session 065: Cloud Files Automated Tests

## Meta
- **Date:** 2026-01-29
- **Goal:** Creer un outil CLI de test automatise pour valider hydratation/deshydratation
- **Status:** Complete

## Current Module
**Working on:** Module 3 - Fix des tests echoues
**Progress:** Tous les 8 tests passent

## Module Checklist
- [x] Module 1: Structure de base + config + scenarios + runner
- [x] Module 2: Tests initiaux T1-T8 (3 PASS, 5 FAIL)
- [x] Module 3: Corrections sync bidirectionnel (8/8 PASS)

## Environnement de test
- **Local sync root:** `D:\cftest`
- **Remote:** `\\192.168.83.221\data_franck\test_anemone\_cftest\`
- **Fichiers sources:** `D:\temp\`

## Resultats finaux
| ID | Scenario | Resultat | Duree |
|----|----------|----------|-------|
| T1 | Hydratation basique | PASS | 35ms |
| T2 | Deshydratation basique | PASS | 23ms |
| T3 | Cycle complet | PASS | 23ms |
| T4 | Upload local | PASS | 12ms |
| T5 | Structure imbriquee | PASS | 72ms |
| T6 | Gros fichier | PASS | 6654ms |
| T7 | Modif serveur | PASS | 52ms |
| T8 | Suppression serveur | PASS | 41ms |

## Corrections appliquees

### 1. SMB Upload de Cloud Files (client_ops.go)
- Remplace verification `IsRegular()` par `IsDir()`
- Permet l'upload de placeholders hydrates (reparse points)

### 2. RunSync bidirectionnel (runner.go)
- Upload si local plus recent OU taille differente (local modifie)
- Download si remote plus recent ET taille differente
- Distinction correcte IsDehydrated (PARTIAL) vs IsPlaceholder

### 3. VerifyFileDehydrated avec retry
- Ajout retry (5x, 200ms) pour gerer les race conditions

### 4. CleanupTestDir()
- Nettoie local ET distant avant chaque execution

## Files Created/Modified
- `cmd/cloudfiles_test/main.go` - CLI entry point (--list, --run, --run-all)
- `cmd/cloudfiles_test/config.go` - Configuration (SMB, paths)
- `cmd/cloudfiles_test/scenarios.go` - 8 scenarios de test (T1-T8)
- `cmd/cloudfiles_test/runner.go` - Execution des tests + helpers (sync bidirectionnel)
- `internal/smb/client_ops.go` - Fix upload de Cloud Files placeholders

## Compilation
```bash
export PATH="/c/msys64/mingw64/bin:$PATH" && go build -o cloudfiles_test.exe ./cmd/cloudfiles_test/
```

## Usage
```bash
./cloudfiles_test.exe --list              # Lister les scenarios
./cloudfiles_test.exe --run-all -v        # Executer tous les tests
./cloudfiles_test.exe --run T1,T2 -v      # Tests specifiques
./cloudfiles_test.exe --reconfig          # Reconfigurer
./cloudfiles_test.exe --cleanup           # Nettoyer apres tests
```
