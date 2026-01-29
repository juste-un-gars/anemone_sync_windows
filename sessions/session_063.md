# Session 063: Dehydration UI - Libérer de l'espace

## Meta
- **Date:** 2026-01-29
- **Goal:** Ajouter fonctionnalité "Libérer de l'espace" via menu systray et CLI
- **Status:** In Progress - Blocked on CfDehydratePlaceholder

## Plan

### Palier 1: Scanner les fichiers hydratés ✓
- Existait déjà dans `dehydration_scan.go`

### Palier 2: UI Dialog "Libérer de l'espace" ✓
- `dehydrate_dialog.go` créé
- Liste fichiers, slider jours, taille totale, confirmation

### Palier 3: Menu System Tray ✓
- Menu "Free Up Space" ajouté à `tray.go`
- Sous-menu avec jobs Files On Demand

### Palier 4: CLI
- TODO: `anemonesync --dehydrate <job-id> [--days 30]`

## Current Issue
**Working on:** Fix CfDehydratePlaceholder qui retourne SUCCESS mais ne déhydrate pas

**Analyse:**
- `CfDehydratePlaceholder` retourne HRESULT=0x00000000 (SUCCESS)
- Mais attributs fichier restent identiques (0x620 avant/après)
- state=0x21 = PLACEHOLDER + PARTIALLY_ON_DISK (hydraté)

**Tentatives:**
1. CreateFile normal → SUCCESS mais pas d'effet
2. CfOpenFileWithOplock → handle "protégé" pas compatible Win32 APIs
3. CfGetWin32HandleFromProtectedHandle → EN TEST

## Files Modified
- `internal/app/dehydrate_dialog.go` - NEW: UI dialog
- `internal/app/tray.go` - Menu "Free Up Space"
- `internal/cloudfiles/cfapi.go` - Ajout procs oplock
- `internal/cloudfiles/cfapi_operations.go` - OpenFileWithOplock, GetWin32Handle
- `internal/cloudfiles/cfapi_placeholder.go` - Debug DehydratePlaceholder
- `internal/cloudfiles/dehydration_scan.go` - DehydrateFile avec oplock
- `internal/cloudfiles/provider.go` - GetDehydrationManager()
- `internal/app/syncmanager_providers.go` - GetProvider()

## Handoff Notes
- Le build compile OK
- Dernière version utilise CfGetWin32HandleFromProtectedHandle
- À tester: lancer l'app et "Free Up Space" pour voir si ça fonctionne maintenant
- Si ça ne fonctionne pas, investiguer autres pistes:
  - Vérifier si le fichier est "pinned" (bloque déhydration)
  - Essayer CF_DEHYDRATE_FLAG_BACKGROUND
  - Vérifier documentation CloudMirror sample

