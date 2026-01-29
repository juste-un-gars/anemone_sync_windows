# Session 049: Debug Cloud Files API

## Meta
- **Date:** 2026-01-27
- **Goal:** Investigation ERROR_CLOUD_FILE_NOT_UNDER_SYNC_ROOT
- **Status:** Partial

## Summary
- Investigation approfondie du probleme
- Probleme identifie: callbacks Go incompatibles avec threading Windows

## Technical Discovery
- Les callbacks Go ne peuvent pas etre appeles depuis les threads Windows du Cloud Filter
- Le Go scheduler interfere avec les threads Windows
