# Session 073: Fix Cloud Files - Hydration massive + ACK_DATA + Reconnexion SMB

## Meta
- **Date:** 2026-02-15
- **Goal:** Corriger 3 bugs critiques Cloud Files identifiés via analyse des logs
- **Status:** Complete

## Problèmes identifiés (analyse logs 10MB)

### Bug 1: Scanner déclenche l'hydratation de TOUS les placeholders
- **Cause**: `scanner/hash.go` ouvre les fichiers avec `os.Open()` pour calculer SHA256
- **Impact**: Pour chaque placeholder, Windows déclenche FETCH_DATA → téléchargement depuis SMB
- **Résultat**: 43 315 fichiers téléchargés au lieu de rester en mode cloud
- **Fix**: Détection des placeholders via `FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS` (0x400000)
  - Fichiers: `scanner/placeholder_windows.go`, `placeholder_other.go`
  - Modifié: `scanner/metadata.go` (champ `IsPlaceholder`)
  - Modifié: `scanner/scanner.go` (skip hashing si placeholder)

### Bug 2: ACK_DATA appelé à tort après FETCH_DATA
- **Cause**: `CfapiBridgeTransferComplete` (ACK_DATA) appelé après TransferData réussi
- **Impact**: 1 491 erreurs `result=-5` dans les logs, métadonnées corrompues (2 014 fichiers)
- **Fix**: ACK_DATA est uniquement pour VALIDATE_DATA, pas FETCH_DATA
  - TransferData avec MARK_IN_SYNC sur le dernier chunk suffit
  - Modifié: `cloudfiles/cfapi_bridge.go` (suppression appel ACK_DATA)

### Bug 3: Connexion SMB non reconnectée pour hydratation
- **Cause**: La connexion SMB pour l'hydratation à la demande tombe (EOF) et ne se reconnecte jamais
- **Impact**: Ouverture de fichier échoue en permanence jusqu'au redémarrage
- **Fix**: Wrapper `reconnectableSMBDataSource` avec auto-reconnexion
  - Modifié: `app/syncmanager_providers.go`

## Files Modified
- `internal/scanner/placeholder_windows.go` - NEW: Détection placeholders Cloud Files
- `internal/scanner/placeholder_other.go` - NEW: Stub non-Windows
- `internal/scanner/metadata.go` - Ajout champ `IsPlaceholder`
- `internal/scanner/scanner.go` - Skip hashing pour placeholders
- `internal/cloudfiles/cfapi_bridge.go` - Suppression ACK_DATA après FETCH_DATA
- `internal/app/syncmanager_providers.go` - SMB reconnectable + manifest support

## Technical Decisions
- **Placeholder detection via file attributes**: Plus fiable que vérifier le type de job, car ça protège même si un fichier est accidentellement un placeholder
- **Pas d'ACK_DATA après FETCH_DATA**: Conforme à la spec Microsoft Cloud Mirror
- **Reconnexion lazy**: Une seule tentative de reconnexion par erreur, pas de retry infini

## Build & Release
- **Commit**: 1b39528
- **Release**: v0.1.2-dev (remplace v0.1.1-dev défectueuse)
- **Build flags**: `-ldflags "-s -w -H windowsgui"` (30 Mo, pas de console, pas de symboles debug)

## Handoff Notes
- Les 2 014 fichiers avec métadonnées corrompues nécessitent `anemone-cleanup`
- Le fix `hydration.go` (path stripping) de la session précédente est conservé (non committé avant)
- **Rappel build**: toujours `-s -w` (taille) et `-H windowsgui` (pas de CMD)
