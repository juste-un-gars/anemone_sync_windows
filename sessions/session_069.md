# Session 069: Améliorations UX (notifications, icônes status)

## Meta
- **Date:** 2026-01-30
- **Goal:** Icônes systray dynamiques, tooltip, progress détaillé + bugfixes
- **Status:** Complete

## Modules implementés

### Module 1: Icônes systray dynamiques
- Nouveau fichier `internal/app/tray_icons.go`
- Génère 4 variantes d'icônes avec badges colorés :
  - **Normal** : icône de base (pas de badge)
  - **Syncing** : badge bleu (Material Blue #2196F3)
  - **Error** : badge rouge (Material Red #F44336)
  - **Warning** : badge orange (Material Orange #FF9800)
- Badge circulaire dans le coin inférieur droit avec bordure blanche
- Changement automatique basé sur le status dans `UpdateStatus()`

### Module 2: Tooltip systray
- Utilise `fyne.io/systray` pour le tooltip Windows/Mac
- Tooltip affiché au survol : "AnemoneSync - [status]"
- Mis à jour en temps réel avec chaque changement de status

### Module 3: Progress détaillé
- Format amélioré : "Syncing JobName: 150/500 files (2.3 GB/5.1 GB)"
- Affiche les fichiers traités ET la taille transférée
- Nouvelle fonction `formatSyncProgress()` dans syncmanager.go

## Files Modified

- `internal/app/tray_icons.go` - **NEW** - Génération d'icônes avec badges
- `internal/app/tray.go` - Intégration icônes dynamiques + tooltip
- `internal/app/syncmanager.go` - Progress détaillé avec taille

## Technical Decisions

- **Badges vs overlay complet** : Badge dans le coin est plus visible et moins intrusif
- **Génération programmatique** : Pas besoin de créer des assets manuels
- **fyne.io/systray** : Seule solution pour tooltip systray avec Fyne

## Bugfixes supplémentaires

### Fix: Sync automatique au démarrage pour FilesOnDemand
- `app.go` : Jobs avec FilesOnDemand synchronisent toujours au démarrage (pas seulement en autostart)

### Fix: Suppression locale avec FilesOnDemand
- `detector.go` : Nouvelle fonction `filesContentSame()` qui compare uniquement taille+hash (ignore mtime)
- Résout le bug où les fichiers supprimés localement redevenaient des placeholders au lieu d'être supprimés sur le serveur

### Fix: Progress avec taille (BytesTotal)
- `executor.go` et `worker_pool.go` : Calcul de BytesTotal pour afficher la progression en taille

## Files Modified (complet)

- `internal/app/tray_icons.go` - **NEW** - Génération d'icônes avec badges
- `internal/app/tray.go` - Icônes dynamiques + tooltip
- `internal/app/syncmanager.go` - Progress détaillé avec taille + log INFO temporaire
- `internal/app/app.go` - Sync au démarrage pour FilesOnDemand
- `internal/cache/detector.go` - filesContentSame() pour fix suppression
- `internal/sync/executor.go` - BytesTotal dans progress callback
- `internal/sync/worker_pool.go` - BytesTotal dans progress callback

## Bugs connus (à corriger)

1. **Crash GLFW à la fermeture** - Panic si fenêtre Settings ouverte au quit
2. **Log INFO temporaire** - À retirer après validation du progress

## Test procedure

1. Lancer l'application
2. Survoler l'icône systray → doit afficher "AnemoneSync - Idle"
3. Déclencher un sync → icône doit avoir un badge bleu + tooltip mis à jour
4. Observer le status dans le menu → doit afficher "Syncing: X/Y files (size)"
5. Si erreur → icône doit avoir un badge rouge
6. **Suppression locale** → fichiers doivent être supprimés sur le serveur (pas recréés)
