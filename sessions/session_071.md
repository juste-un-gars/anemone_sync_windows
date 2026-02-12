# Session 071: Config Export/Import + First Release

## Meta
- **Date:** 2026-02-12
- **Goal:** Ajouter export/import de la configuration + premiere release GitHub
- **Status:** Complete

## Completed Modules

| Module | Validated | Date |
|--------|-----------|------|
| DB: GetAllExclusions + CreateExclusion | Yes | 2026-02-12 |
| config_export.go (ExportConfig/ImportConfig) | Yes | 2026-02-12 |
| Settings UI: Export/Import buttons | Yes | 2026-02-12 |
| Build release (icon + windowsgui) | Yes | 2026-02-12 |
| GitHub Release v0.1.0-dev | Yes | 2026-02-12 |

## Technical Decisions
- **Format JSON v1**: Versionne pour compatibilite future
- **Pas de mots de passe**: Securite - credentials restent dans Windows Credential Manager
- **Deduplication a l'import**: Skip serveurs (meme host), jobs (meme local+remote path), exclusions (meme pattern+job)
- **Remap IDs**: Les IDs changent a l'import, mapping ancien->nouveau pour exclusions

## Files Modified
- `internal/database/db_jobs.go` - Ajout GetAllExclusions(), CreateExclusion()
- `internal/app/config_export.go` - NEW: Export/Import logic (~130 lignes)
- `internal/app/settings.go` - Ajout section Backup/Restore dans General tab

## Handoff Notes
- Release v0.1.0-dev publiee sur GitHub avec le .exe
- Le .exe est standalone (icon embarquee, pas de console, DLLs statiques)
- Prochaine etape: tests utilisateur en conditions reelles
