# Client de Synchronisation SMB Multi-Plateforme

## Vue d'ensemble du projet

Application de synchronisation de fichiers vers partages SMB, fonctionnant comme OneDrive mais avec des serveurs SMB au lieu du cloud. Multi-plateforme (Windows prioritaire, puis Linux, Android, iOS).

## Objectifs principaux

- Synchronisation temps réel ou planifiée vers partages SMB
- Multi-serveurs et multi-mappings
- Sécurité maximale (zéro mot de passe en clair)
- Performance optimale (sync incrémentale)
- Interface utilisateur intuitive

---

## Fonctionnalités détaillées

### 1. Gestion des connexions SMB

#### Multi-serveurs
- Possibilité de se connecter à plusieurs serveurs SMB simultanément
- Chaque serveur a ses propres credentials
- Format de connexion : `\\serveur\partage` ou `smb://serveur/partage`

#### Credentials sécurisés
- **Stockage plateforme-spécifique** :
  - Windows : Credential Manager (DPAPI)
  - Linux : Secret Service API (libsecret, Gnome Keyring, KDE Wallet)
  - Android : Android Keystore System
  - iOS : Keychain Services
- **Aucun mot de passe en clair** nulle part
- Zérotisation mémoire après usage
- Pas de logs contenant des credentials

### 2. Mapping et synchronisation

#### Configuration des mappings
- **Mapping flexible** : N dossiers locaux ↔ N chemins SMB
- Chaque mapping est un "job" indépendant avec ses propres paramètres
- Interface de parcours des dossiers locaux et distants

#### Modes de synchronisation (par job)
1. **Miroir bidirectionnel** : Les deux côtés restent identiques
2. **Upload only** : Local → SMB (le local écrase le distant)
3. **Download only** : SMB → Local (le distant écrase le local)
4. **Miroir avec priorité** : Bidirectionnel avec règle de priorité en cas de conflit

#### Gestion des conflits (mode miroir)
Paramètre utilisateur avec options :
- Le plus récent gagne (par timestamp)
- Le local gagne toujours
- Le distant gagne toujours
- Renommer et garder les deux (.local / .remote)
- Demander à chaque fois (popup de décision)

#### Gestion des suppressions
Options configurables par job :
- Propager les suppressions (local → distant et/ou inverse)
- Ne pas propager (garder les fichiers)
- Corbeille virtuelle optionnelle

### 3. Modes de déclenchement

#### Temps réel
1. **Immédiat** : Dès détection du changement → sync
2. **Avec délai (debouncing)** : Attend X secondes après le dernier changement
   - Paramètre par défaut : 30 secondes
   - Évite de syncer un fichier en cours d'édition
3. **Groupé (batch)** : Accumule changements pendant X minutes puis sync en batch
   - Paramètre par défaut : 5 minutes
   - Optimisé pour dossiers avec nombreuses petites modifications

#### Planifié
1. **Intervalle régulier** : Toutes les X minutes/heures
2. **Horaire spécifique** : À des heures précises (ex: 2h, 14h, 20h)
3. **Hybride** : Temps réel mais seulement dans certaines plages horaires

#### Manuel
- Déclenchement sur demande de l'utilisateur
- Bouton "Synchroniser maintenant" par job ou global

### 4. Exclusions et filtres

#### Trois niveaux d'exclusions

**1. Globales** (tous les jobs)
- Patterns par défaut :
  - `*.tmp`
  - `~$*` (fichiers temp Office)
  - `.DS_Store` (macOS)
  - `Thumbs.db` (Windows)
  - `desktop.ini` (Windows)
  - `~lock.*` (fichiers de verrouillage)
- Extensions personnalisées ajoutables
- Patterns wildcards : `*.log`, `temp/*`, etc.

**2. Par job** (spécifique à un mapping)
- Mêmes options que global mais uniquement pour ce job
- Priorité sur les règles globales

**3. Individuelles** (fichiers/dossiers spécifiques)
- Liste de chemins absolus à ignorer
- Ajoutés manuellement ou automatiquement suite à erreur
- Chaque exclusion stocke la raison et la date

#### Gestion des erreurs de sync
Workflow en cas d'erreur :
```
Sync en cours → Erreur sur fichier.xyz
├─ Log l'erreur avec détails complets
├─ Passe au fichier suivant (ne bloque pas le reste)
├─ Notification/popup :
│  ❌ Erreur : Impossible de syncer C:\...\fichier.xyz
│  Raison : [message d'erreur détaillé]
│  
│  [Réessayer]  [Ignorer cette fois]  [Toujours ignorer ce fichier]
│
└─ Si "Toujours ignorer" → Ajoute à la liste d'exclusions individuelles
```

Cas d'erreurs typiques à gérer :
- Fichier verrouillé/en cours d'utilisation
- Permissions insuffisantes
- Nom de fichier invalide pour SMB
- Fichier trop volumineux (si limite configurée)
- Connexion SMB perdue
- Espace disque insuffisant

### 5. Gestion réseau et connectivité

#### Conditions de synchronisation (priorité mobile)

**Basique** (à implémenter en priorité)
- Sync uniquement si WiFi
- Sync uniquement si Data mobile
- Sync sur WiFi ET/OU Data
- Mode hors-ligne : file d'attente des modifications

**Avancé** (pour version ultérieure)
- Liste blanche de réseaux WiFi spécifiques (SSID)
- Détection réseau local/RJ45 spécifique
- Règles par job (ex: photos uniquement en WiFi, documents en WiFi+Data)

#### Comportement hors-ligne
- Détection automatique du type de connexion
- Si conditions non remplies → ajout à la file d'attente
- File d'attente persistante (survit au redémarrage)
- Notification optionnelle : "X fichiers en attente de WiFi"
- Reprise automatique quand conditions OK

#### Reprise après crash
- Fichier partiellement transféré → retransfer complet (plus sûr)
- État des fichiers en attente préservé dans DB SQLite
- Reprise automatique au redémarrage de l'application
- Log de crash pour diagnostic

### 6. Optimisation et performance

#### Synchronisation incrémentale

**Base de données d'état (SQLite chiffré avec SQLCipher)**
```sql
Table: files_state
- id (PRIMARY KEY)
- job_id (référence au job de sync)
- local_path (chemin absolu local)
- remote_path (chemin SMB)
- size (taille en bytes)
- mtime (timestamp de modification)
- hash (SHA256 du contenu)
- last_sync (timestamp dernière sync)
- sync_status (idle/syncing/error/queued)
- error_message (si erreur)

Table: exclusions
- id (PRIMARY KEY)
- type (global/job/individual)
- pattern_or_path
- reason (optionnel)
- date_added
- job_id (NULL si global)

Table: sync_jobs
- id (PRIMARY KEY)
- name (nom du job)
- local_path
- remote_path
- server_credential_id (référence au keystore)
- sync_mode (mirror/upload/download/mirror_priority)
- trigger_mode (realtime/interval/scheduled/manual)
- trigger_params (JSON : délai, intervalle, horaires, etc.)
- conflict_resolution (recent/local/remote/both/ask)
- network_conditions (JSON : wifi, data, specific_networks)
- enabled (boolean)
- last_run
- next_run

Table: sync_history
- id (PRIMARY KEY)
- job_id
- timestamp
- files_synced
- files_failed
- bytes_transferred
- duration
- status (success/partial/failed)
```

**Détection des changements**
1. Vérification rapide : taille + date modification (mtime)
2. Si changement potentiel détecté → calcul hash SHA256
3. Comparaison hash avec DB pour confirmer modification réelle
4. Seuls les fichiers réellement modifiés sont transférés

**Transfert optimisé**
- Parallélisation : plusieurs fichiers en simultané (goroutines Go)
- Nombre de threads configurables (défaut : 4-8)
- Compression à la volée optionnelle
- Delta sync (optionnel, phase 2) : rsync-like, transfert uniquement des blocs modifiés

**Performance système**
- Application la plus légère possible
- Consommation RAM optimisée
- CPU : utilisation minimale en idle, burst lors des syncs
- Pas de polling agressif du filesystem
- File system watchers natifs (Windows: ReadDirectoryChangesW, Linux: inotify)

#### Throttling de bande passante
- Limite configurable en KB/s ou MB/s
- Par job ou global
- Plages horaires : vitesse différente selon l'heure
- Pas de limite = vitesse maximale

### 7. Interface utilisateur

#### Icône systray
- **États visuels** :
  - Vert : Idle, tout OK
  - Bleu animé : Sync en cours
  - Orange : En attente (conditions réseau non remplies)
  - Rouge : Erreur nécessitant attention
- **Menu contextuel** :
  - Synchroniser maintenant
  - Pause / Resume
  - Ouvrir configuration
  - Voir logs
  - Statistiques
  - Quitter

#### Fenêtre principale
**Onglets** :

1. **Jobs** : Liste des mappings configurés
   - Nom, Local ↔ Remote, Mode, État, Dernière sync
   - Actions : Éditer, Supprimer, Sync Now, Pause/Resume

2. **Nouveau Job** : Assistant de création
   - Étape 1 : Sélection serveur SMB (existant ou nouveau)
   - Étape 2 : Parcours et sélection dossier local
   - Étape 3 : Parcours et sélection dossier distant
   - Étape 4 : Configuration (mode sync, déclenchement, filtres)

3. **Serveurs** : Gestion des serveurs SMB
   - Liste des serveurs configurés
   - Ajouter, éditer, supprimer, tester connexion

4. **Exclusions** : Gestion des exclusions
   - Sous-onglets : Globales / Par job / Individuelles
   - Pour les individuelles : afficher raison et date

5. **Logs** : Visualisation des logs
   - Filtres par niveau (debug/info/warning/error)
   - Filtres par job
   - Recherche textuelle
   - Export logs

6. **Paramètres** : Configuration globale
   - Notifications (toutes paramétrables ON/OFF)
   - Performance (threads, RAM, throttling)
   - Langue
   - Logs (niveau, rotation)
   - Démarrage automatique avec système
   - Mise à jour

#### Notifications
Toutes paramétrables individuellement (ON/OFF) :
- Sync terminée avec succès
- Erreurs critiques
- Conflits détectés
- Fichiers en attente de réseau
- Espace disque faible

Niveaux de logs :
- Debug (très verbeux, pour développement)
- Info (événements normaux)
- Warning (situations anormales mais gérées)
- Error (erreurs nécessitant attention)
- Critical (erreurs bloquantes)

### 8. Sécurité (CRITIQUE)

#### Principes de base
- **ZÉRO mot de passe en clair** nulle part
- Chiffrement de bout en bout pour le stockage
- Zérotisation mémoire après usage des credentials
- Pas de transmission non chiffrée (SMB utilise déjà SMB3 encryption si disponible)

#### Stockage credentials
- Utilisation des keystores système natifs
- Library Go : `github.com/zalando/go-keyring` (cross-platform)
- Stockage uniquement d'un ID dans la config, credentials dans keystore

#### Base de données
- SQLCipher pour chiffrement complet de la DB
- Clé de chiffrement stockée dans le keystore système
- Même la DB ne contient jamais de credentials

#### Configuration
- Fichiers de config en JSON ou YAML
- Ne contiennent que des références (IDs) aux credentials
- Permissions fichiers restrictives (600 sur Unix, ACL sur Windows)

### 9. Logs et monitoring

#### Système de logs
- **Rotation automatique** :
  - Taille maximale par fichier : 10 MB
  - Nombre de fichiers conservés : 5
  - Compression des anciens logs (.gz)
- **Emplacement** :
  - Windows : `%APPDATA%\SMBSync\logs\`
  - Linux : `~/.config/smbsync/logs/`
  - macOS : `~/Library/Application Support/SMBSync/logs/`
- **Format** : timestamp, niveau, job_id, message

#### Métriques et statistiques
- Nombre total de fichiers synchronisés
- Volume de données transférées
- Temps moyen de sync
- Taux de succès/échec
- Historique sur 7/30/90 jours

### 10. Internationalisation (i18n)

#### Architecture
- Fichiers de traduction séparés (JSON ou YAML)
- Structure hiérarchique :
  ```json
  {
    "ui": {
      "buttons": {
        "save": "Enregistrer",
        "cancel": "Annuler"
      }
    },
    "errors": {
      "connection_failed": "Connexion échouée"
    }
  }
  ```

#### Langues
- **Phase 1** : Français (langue par défaut)
- **Phase 2** : Anglais
- **Phase 3+** : Autres langues selon demande

#### Détection
- Détection automatique de la langue système
- Possibilité de changer manuellement dans les paramètres

### 11. Mise à jour

#### Phase 1 : Manuel
- Vérification de nouvelle version au démarrage (optionnel)
- Notification si nouvelle version disponible
- Lien vers page de téléchargement

#### Phase 2 : Automatique (futur)
- Téléchargement en arrière-plan
- Installation au prochain redémarrage
- Possibilité de reporter ou désactiver

---

## Architecture technique

### Stack technologique

#### Langage principal : Go
**Avantages** :
- Cross-platform natif (Windows, Linux, macOS, Android, iOS)
- Compilation en binaires standalone (pas de runtime)
- Excellente gestion de la concurrence (goroutines)
- Performance native
- Écosystème riche pour SMB, crypto, GUI

#### Base de données : SQLite + SQLCipher
- Embarquée, pas de serveur séparé
- Chiffrée (SQLCipher)
- Légère et performante
- Library Go : `github.com/mutecomm/go-sqlcipher`

#### Sécurité
- Keystore : `github.com/zalando/go-keyring`
- Crypto : package `crypto` standard de Go (SHA256, AES)
- SMB3 encryption native quand disponible

#### Interface graphique
**Options à évaluer** :
1. **Fyne** (`fyne.io`) : Go pur, cross-platform, moderne
2. **Wails** : Go backend + frontend web (React/Vue/Svelte)
3. **Qt bindings** : Plus mature mais plus lourd

**Recommandation initiale** : Fyne pour rester en Go pur

#### SMB/CIFS
- Library : `github.com/hirochachacha/go-smb2` (SMB2/3)
- Fallback : commandes natives système si nécessaire

#### File System Watching
- Windows : `golang.org/x/sys/windows` (ReadDirectoryChangesW)
- Linux : `github.com/fsnotify/fsnotify` (inotify)
- macOS : fsnotify (FSEvents)

### Architecture modulaire

```
smbsync/
├── cmd/
│   └── smbsync/
│       └── main.go              # Point d'entrée
├── internal/
│   ├── config/                  # Gestion configuration
│   │   ├── config.go
│   │   └── validation.go
│   ├── credential/              # Gestion credentials
│   │   ├── keystore.go
│   │   └── manager.go
│   ├── database/                # Base SQLite
│   │   ├── db.go
│   │   ├── migrations.go
│   │   └── models.go
│   ├── sync/                    # Moteur de synchronisation
│   │   ├── engine.go            # Orchestration
│   │   ├── scanner.go           # Détection changements
│   │   ├── transfer.go          # Transfert fichiers
│   │   ├── conflict.go          # Résolution conflits
│   │   └── queue.go             # File d'attente
│   ├── smb/                     # Client SMB
│   │   ├── client.go
│   │   ├── connection.go
│   │   └── operations.go
│   ├── watcher/                 # File system watching
│   │   ├── watcher.go
│   │   └── debouncer.go
│   ├── network/                 # Détection réseau
│   │   └── detector.go
│   ├── scheduler/               # Planification
│   │   └── scheduler.go
│   ├── exclusion/               # Gestion exclusions
│   │   └── manager.go
│   ├── ui/                      # Interface graphique
│   │   ├── main_window.go
│   │   ├── tray.go
│   │   ├── jobs_tab.go
│   │   ├── servers_tab.go
│   │   ├── exclusions_tab.go
│   │   ├── logs_tab.go
│   │   └── settings_tab.go
│   ├── notification/            # Notifications système
│   │   └── notifier.go
│   ├── i18n/                    # Internationalisation
│   │   ├── translator.go
│   │   └── locales/
│   │       ├── fr.json
│   │       └── en.json
│   └── logger/                  # Système de logs
│       ├── logger.go
│       └── rotation.go
├── pkg/                         # Packages réutilisables
│   └── utils/
│       ├── hash.go
│       ├── path.go
│       └── compress.go
├── configs/                     # Configs par défaut
│   └── default_exclusions.json
├── build/                       # Scripts de build
│   ├── windows.sh
│   ├── linux.sh
│   └── darwin.sh
├── docs/                        # Documentation
│   ├── ARCHITECTURE.md
│   ├── USER_GUIDE.md
│   └── API.md
├── go.mod
├── go.sum
├── README.md
├── LICENSE
└── CLAUDE.md                    # Ce fichier
```

### Flow général

#### Au démarrage de l'application
1. Chargement configuration depuis fichier
2. Initialisation DB SQLite (création tables si nécessaire)
3. Récupération credentials depuis keystore système
4. Démarrage service de synchronisation pour chaque job actif
5. Initialisation watchers filesystem si mode temps réel
6. Affichage UI (systray + fenêtre si demandé)

#### Pour chaque job de synchronisation

**Mode temps réel** :
```
Watcher détecte changement
    ↓
Debouncer (si configuré)
    ↓
Vérification conditions réseau
    ↓ (OK)                      ↓ (KO)
Scanner identifie fichiers   → File d'attente
    ↓
Calcul hash si nécessaire
    ↓
Vérification exclusions
    ↓
Transfert vers SMB
    ↓
Mise à jour DB
    ↓
Notification (si configurée)
```

**Mode planifié/manuel** :
```
Trigger (cron ou manuel)
    ↓
Scan complet dossier local + distant
    ↓
Comparaison avec DB (hash, mtime, size)
    ↓
Identification changements (nouveau/modifié/supprimé)
    ↓
Vérification conditions réseau
    ↓ (OK)                      ↓ (KO)
Résolution conflits         → File d'attente
    ↓
Vérification exclusions
    ↓
Transfert batch
    ↓
Mise à jour DB
    ↓
Notification + stats
```

---

## Phases de développement

### Phase 0 : Setup et architecture (ACTUEL)
- [ ] Initialisation projet Go
- [ ] Structure de dossiers
- [ ] Configuration de base (viper ou autre)
- [ ] Setup SQLite + SQLCipher
- [ ] Tests de connexion SMB basique

### Phase 1 : Core - Moteur de synchronisation
- [ ] Modèle de données (DB schema)
- [ ] Scanner de fichiers (local + SMB)
- [ ] Calcul hash et détection changements
- [ ] Transfert fichiers basique (un par un)
- [ ] Gestion exclusions (globales d'abord)
- [ ] Tests unitaires

### Phase 2 : Sécurité et credentials
- [ ] Intégration keystore système (go-keyring)
- [ ] Chiffrement SQLite (SQLCipher)
- [ ] Gestion sécurisée credentials SMB
- [ ] Tests de sécurité

### Phase 3 : Modes de synchronisation
- [ ] Implémentation mode miroir bidirectionnel
- [ ] Implémentation upload only
- [ ] Implémentation download only
- [ ] Gestion conflits
- [ ] Tests de scénarios

### Phase 4 : Déclenchement et planification
- [ ] File system watcher (temps réel)
- [ ] Debouncer pour temps réel avec délai
- [ ] Scheduler pour modes planifiés
- [ ] File d'attente hors-ligne
- [ ] Tests de planification

### Phase 5 : Interface utilisateur (Windows)
- [ ] Setup Fyne
- [ ] Fenêtre principale avec onglets
- [ ] Icône systray + menu contextuel
- [ ] Assistant création job
- [ ] Gestion serveurs SMB
- [ ] Interface exclusions
- [ ] Visualisation logs
- [ ] Paramètres globaux

### Phase 6 : Performance et optimisation
- [ ] Parallélisation transferts
- [ ] Throttling bande passante
- [ ] Optimisation mémoire
- [ ] Monitoring performance
- [ ] Tests de charge

### Phase 7 : Réseau et mobilité
- [ ] Détection type connexion réseau
- [ ] Conditions WiFi/Data
- [ ] File d'attente hors-ligne robuste
- [ ] Reprise après crash
- [ ] Tests réseau

### Phase 8 : Notifications et UX
- [ ] Système de notifications cross-platform
- [ ] Statistiques et historique
- [ ] Export logs
- [ ] Tests UX

### Phase 9 : Internationalisation
- [ ] Infrastructure i18n
- [ ] Traduction française complète
- [ ] Traduction anglaise
- [ ] Tests linguistiques

### Phase 10 : Packaging et distribution
- [ ] Build scripts cross-platform
- [ ] **Installeur Windows .exe avec NSIS** (prioritaire)
  - Interface graphique complète
  - Sélection répertoire d'installation
  - Création raccourcis (Menu Démarrer, Bureau)
  - Option démarrage automatique Windows
  - Désinstalleur avec option conservation des données
  - Voir [docs/INSTALLER.md](docs/INSTALLER.md) pour détails complets
- [ ] Package Linux (.deb, .rpm, AppImage)
- [ ] Signature code (certificat pour Windows)
- [ ] Tests installation sur Windows 10/11
- [ ] Automatisation avec GitHub Actions

### Phase 11+ : Portage mobile et fonctionnalités avancées
- [ ] Portage Android
- [ ] Portage iOS
- [ ] Delta sync (rsync-like)
- [ ] Réseaux WiFi spécifiques
- [ ] Auto-update
- [ ] Compression avancée

---

## Considérations importantes

### Sécurité
- **CRITIQUE** : Aucun mot de passe ne doit jamais être stocké ou loggé en clair
- Audit de sécurité régulier
- Gestion sécurisée de la mémoire (zérotisation)
- Permissions fichiers restrictives

### Performance
- Application doit rester légère même avec milliers de fichiers
- Utilisation CPU minimale en idle
- Optimisation I/O disque
- Gestion intelligente de la mémoire

### Compatibilité
- Tester sur différentes versions Windows (10, 11)
- Tester sur différentes distributions Linux
- Supporter différentes versions SMB (2.x, 3.x)
- Gérer caractères spéciaux dans noms de fichiers

### Robustesse
- Gestion d'erreurs exhaustive
- Récupération après crash
- Validation des données
- Logs détaillés pour debugging

### UX
- Interface intuitive pour utilisateurs non techniques
- Feedback visuel clair
- Messages d'erreur compréhensibles
- Documentation utilisateur complète

---

## Commandes utiles

### Développement
```bash
# Run
go run cmd/smbsync/main.go

# Build
go build -o smbsync cmd/smbsync/main.go

# Tests
go test ./...

# Tests avec coverage
go test -cover ./...

# Linter
golangci-lint run
```

### Build release
```bash
# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o smbsync.exe cmd/smbsync/main.go

# Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o smbsync cmd/smbsync/main.go

# macOS
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o smbsync cmd/smbsync/main.go
```

---

## Dépendances Go principales

```go
require (
    github.com/hirochachacha/go-smb2 v1.1.0           // Client SMB
    github.com/mutecomm/go-sqlcipher/v4 v4.4.2        // SQLite chiffré
    github.com/zalando/go-keyring v0.2.3               // Keystore cross-platform
    github.com/fsnotify/fsnotify v1.7.0                // File system watcher
    fyne.io/fyne/v2 v2.4.0                             // GUI cross-platform
    github.com/robfig/cron/v3 v3.0.1                   // Scheduler
    github.com/spf13/viper v1.18.2                     // Configuration
    go.uber.org/zap v1.26.0                            // Logger performant
    golang.org/x/crypto v0.17.0                        // Crypto supplémentaire
    golang.org/x/sys v0.15.0                           // Appels système natifs
)
```

---

## Ressources et références

### SMB/CIFS
- Protocole SMB : https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb/
- go-smb2 : https://github.com/hirochachacha/go-smb2

### Sécurité
- DPAPI Windows : https://docs.microsoft.com/en-us/windows/win32/api/dpapi/
- Keychain iOS/macOS : https://developer.apple.com/documentation/security/keychain_services
- Android Keystore : https://developer.android.com/training/articles/keystore

### GUI
- Fyne : https://fyne.io/
- Wails : https://wails.io/

### File System
- fsnotify : https://github.com/fsnotify/fsnotify
- Windows ReadDirectoryChanges : https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-readdirectorychangesw

---

## Notes de développement

### Priorités
1. **Sécurité** : Jamais de compromis
2. **Fiabilité** : Ne jamais perdre de données
3. **Performance** : Léger et rapide
4. **UX** : Simple et intuitif

### Bonnes pratiques
- Tests unitaires systématiques
- Documentation du code (godoc)
- Gestion d'erreurs explicite (pas de panic en production)
- Logs structurés
- Versioning sémantique

### Logs de développement
À maintenir dans un fichier CHANGELOG.md séparé

---

## Contact et contribution

### Mainteneur
Franck - Développeur principal

### Licence
À définir (MIT, Apache 2.0, ou autre)

### Contribution
Projet actuellement en développement initial. Guidelines de contribution à venir.

---

**Dernière mise à jour** : 2026-01-10
**Version du document** : 1.0
**Statut** : Spécifications complètes - Prêt pour développement
