# Session 013: Fix CGO

## Meta
- **Date:** 2026-01-14
- **Goal:** Resoudre probleme de compilation CGO
- **Status:** Complete

## Summary
- Decouverte que TDM-GCC produit des binaires corrompus avec Fyne
- MSYS2 MinGW64 GCC requis pour compiler correctement

## Issue & Solution
- **Probleme:** "CETTE APPLICATION NE PEUT PAS S'EXECUTER SUR VOTRE PC"
- **Cause:** TDM-GCC 10.3.0 incompatible avec Fyne CGO
- **Solution:** Utiliser MSYS2 MinGW64 GCC 15.2.0

## Build Command
```bash
export PATH="/c/msys64/mingw64/bin:$PATH" && go build -o anemonesync.exe ./cmd/anemonesync/
```
