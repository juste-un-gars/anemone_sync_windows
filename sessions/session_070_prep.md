# Session 070 - Préparation des tests

## Environnement de test

| Élément | Chemin |
|---------|--------|
| Source fichiers test | `D:\temp` |
| Dossier local sync | `D:\test_anemone` |
| Dossier remote sync | `\\192.168.83.221\data_franck\test_anemone` |
| Job configuré | `test_anemone` (FilesOnDemand activé) |

## Préparation (à faire avant les tests)

1. **Vider complètement les dossiers :**
   ```cmd
   rd /s /q D:\test_anemone
   mkdir D:\test_anemone
   ```

2. **Vider le dossier remote** (via explorateur ou commande) :
   - Supprimer tout dans `\\192.168.83.221\data_franck\test_anemone\`

3. **Préparer des fichiers de test dans D:\temp** :
   - Quelques petits fichiers (< 1 MB) pour tests rapides
   - 1-2 gros fichiers (> 100 MB) pour tester le progress

---

## Scénarios de test

### TEST A : Upload initial
**But :** Vérifier que les fichiers sont uploadés vers le serveur

1. Copier fichiers de `D:\temp` vers `D:\test_anemone`
2. Lancer sync (ou attendre auto-sync)
3. **Vérifier :**
   - [ ] Progress affiché "X/Y files (size)" dans tooltip
   - [ ] Fichiers présents sur le serveur
   - [ ] Status final "Synced X files" ou "Up to date"

### TEST B : Suppression locale → Suppression serveur
**But :** Vérifier le fix de suppression avec FilesOnDemand

1. Supprimer 2-3 fichiers dans `D:\test_anemone`
2. Lancer sync
3. **Vérifier :**
   - [ ] Fichiers supprimés sur le serveur (PAS recréés comme placeholders)
   - [ ] Log montre "delete_remote" actions (pas "download")

### TEST C : Ajout sur serveur → Placeholder local
**But :** Vérifier que les nouveaux fichiers serveur deviennent des placeholders

1. Copier fichiers directement sur `\\192.168.83.221\data_franck\test_anemone\`
2. Lancer sync
3. **Vérifier :**
   - [ ] Fichiers apparaissent dans `D:\test_anemone` comme placeholders (icône nuage)
   - [ ] Double-clic sur placeholder → hydratation (téléchargement)

### TEST D : Modification locale
**But :** Vérifier que les modifications sont uploadées

1. Modifier un fichier dans `D:\test_anemone` (ouvrir, modifier, sauvegarder)
2. Lancer sync
3. **Vérifier :**
   - [ ] Fichier modifié uploadé sur le serveur
   - [ ] Timestamp serveur mis à jour

### TEST E : Icône systray dynamique
**But :** Vérifier les badges de status

1. Observer l'icône systray pendant le sync
2. **Vérifier :**
   - [ ] Badge bleu pendant sync
   - [ ] Badge normal après sync réussi
   - [ ] Badge rouge si erreur (couper réseau pendant sync)

### TEST F : Coupure réseau (résilience)
**But :** Vérifier l'upload atomique

1. Copier un gros fichier (> 500 MB) dans `D:\test_anemone`
2. Lancer sync
3. Pendant l'upload, couper le réseau (désactiver carte réseau)
4. Réactiver le réseau
5. Lancer sync
6. **Vérifier :**
   - [ ] Fichier `.anemone-uploading` sur serveur pendant sync interrompu
   - [ ] Fichier final correct après reprise
   - [ ] Pas de fichier local écrasé

---

## Commandes utiles

```cmd
# Voir contenu local
dir D:\test_anemone

# Voir contenu remote
dir \\192.168.83.221\data_franck\test_anemone

# Copier fichiers de test
copy D:\temp\*.* D:\test_anemone\

# Lancer l'app
E:\AnemoneSync\anemonesync.exe

# Voir les logs en temps réel (PowerShell)
Get-Content E:\AnemoneSync\log.txt -Wait -Tail 50
```

---

## Bugs à surveiller

1. **Crash GLFW à la fermeture** (si fenêtre Settings ouverte) - à corriger
2. **Progress non visible** dans l'UI - tooltip fonctionne, UI à vérifier
