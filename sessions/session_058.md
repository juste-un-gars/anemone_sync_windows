# Session 058: Cloud Files - Fix crash GC Go

## Meta
- **Date:** 2026-01-28
- **Goal:** Fix crash "invalid pointer found on stack"
- **Status:** Partial

## Summary
- Crash corrige (unsafe.Pointer -> uintptr pour HANDLE)
- Fichier bloque, necessite reboot

## Technical Discovery
Le GC Go scanne la stack et voit `unsafe.Pointer`. Quand c'est un HANDLE Windows (petite valeur comme 0xaa0), le GC pense que c'est un pointeur Go invalide et crash.

## Solution
```go
// AVANT (crash):
completionEvent := unsafe.Pointer(req.completionEvent)

// APRES (OK):
completionEvent := uintptr(req.completionEvent)
```
