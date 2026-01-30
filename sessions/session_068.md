# Session 068: Tests de resilience reseau

## Meta
- **Date:** 2026-01-30
- **Goal:** Ajouter des tests de resilience (serveur offline, interruption reseau)
- **Status:** Complete

## Resultat

### Bug decouvert et corrige

**Probleme:** Lors d'un upload interrompu (coupure serveur), le fichier partiel sur le serveur etait considere comme "plus recent" et ecrasait le fichier local complet au prochain sync.

**Solution implementee:** Upload atomique
1. Upload vers `fichier.anemone-uploading` (fichier temporaire)
2. Si succes → rename vers nom final
3. Si echec → fichier `.anemone-uploading` reste (pas de conflit)
4. Au prochain sync → cleanup automatique des `.anemone-uploading` orphelins

### Tests effectues

| Test | Resultat |
|------|----------|
| Coupure serveur pendant upload 2.9GB | Fichier `.anemone-uploading` cree |
| Reprise apres reconnexion | Cleanup + re-upload complet (2.9GB) |
| Pas d'ecrasement fichier local | OK |

### Scenarios TEST7 ajoutes au harness

| ID | Nom | Description |
|----|-----|-------------|
| 7.1 | Serveur offline au demarrage | Sync quand serveur coupe |
| 7.2 | Reprise apres reconnexion | Sync apres remise serveur |
| 7.3 | Gros fichier - preparation | Creer fichier 10MB et sync |
| 7.4 | Interruption pendant sync | Couper pendant transfert |
| 7.5 | Reprise apres interruption | Sync apres remise serveur |

## Files Modified

- `internal/smb/client_ops.go` - Upload atomique (.anemone-uploading + rename)
- `internal/sync/engine_actions.go` - Cleanup des fichiers orphelins au demarrage
- `internal/sync/remote_scanner.go` - Exclusion des fichiers .anemone-uploading
- `test/harness/scenarios.go` - Ajout getResilienceScenarios() (TEST7)
- `test/harness/harness.go` - Mode interactif waitForUser(), ExpectError, SkipSync
- `test/harness/writer.go` - Champ Message pour action wait_user
- `test/harness/validator.go` - Support mode mapped drive
- `test/harness/config.go` - Ajout job TEST7, support mapped drive

## Technical Decisions

- **Upload atomique**: Standard de l'industrie (Dropbox, OneDrive, rsync)
- **Suffixe `.anemone-uploading`**: Prefixe unique pour eviter conflits
- **Cleanup au demarrage**: Nettoie les uploads orphelins avant chaque sync

## Handoff Notes

Le fix d'upload atomique est critique pour la fiabilite du sync. Teste avec fichier de 2.9GB et coupure reseau manuelle.
