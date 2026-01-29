# Session 057: Cloud Files - Architecture synchrone bloquante

## Meta
- **Date:** 2026-01-28
- **Goal:** Nouvelle architecture pour hydration
- **Status:** Partial

## Summary
- Decouverte cause racine: callback doit rester actif pendant CfExecute
- Implementation architecture bloquante avec Event Windows

## Technical Discovery
Le sample CloudMirror appelle CfExecute **depuis le callback**, jamais apres son retour.

## Architecture implementee
1. Callback C recoit FETCH_DATA
2. C cree Event Windows (CreateEventW)
3. C enqueue le message avec Event
4. C bloque sur Event (WaitForSingleObject, 60s timeout)
5. Go traite, lit SMB
6. Go appelle CfExecute (contexte callback actif!)
7. Go signal Event
8. C se reveille, callback retourne
