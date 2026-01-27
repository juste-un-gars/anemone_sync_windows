# AnemoneSync - Client de Synchronisation SMB

[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://golang.org/)
[![Status](https://img.shields.io/badge/status-v1.0%20fonctionnel-green.svg)](https://github.com/juste-un-gars/anemone_sync_windows)

## Vue d'ensemble

AnemoneSync est une application desktop de synchronisation de fichiers vers des partages SMB, fonctionnant comme OneDrive mais avec des serveurs SMB au lieu du cloud.

**Application Windows fonctionnelle** avec interface graphique (Fyne), system tray, et synchronisation bidirectionnelle.

### Objectifs principaux

- 🔄 Synchronisation temps réel ou planifiée vers partages SMB
- 🖥️ Multi-serveurs et multi-mappings
- 🔒 Sécurité maximale (zéro mot de passe en clair)
- ⚡ Performance optimale (synchronisation incrémentale)
- 🎨 Interface utilisateur intuitive

## Fonctionnalités clés

### Gestion des connexions
- Connexion à plusieurs serveurs SMB simultanément
- Credentials sécurisés via keystores système (Credential Manager, Keychain, etc.)
- Support SMB 2.x et 3.x

### Modes de synchronisation
- **Miroir bidirectionnel**: Les deux côtés restent identiques
- **Upload only**: Local → SMB uniquement
- **Download only**: SMB → Local uniquement
- **Miroir avec priorité**: Bidirectionnel avec règles de conflits

### Déclenchement flexible
- **Temps réel**: Synchronisation immédiate ou avec délai (debouncing)
- **Planifié**: Intervalles réguliers ou horaires spécifiques
- **Manuel**: Déclenchement sur demande

### Sécurité
- ✅ Aucun mot de passe en clair stocké
- ✅ Base de données chiffrée (SQLCipher)
- ✅ Zérotisation mémoire après usage
- ✅ Utilisation des keystores natifs de chaque plateforme

### Performance
- Synchronisation incrémentale (hash SHA256)
- Parallélisation des transferts
- Throttling de bande passante configurable
- File system watchers natifs (inotify, FSEvents, ReadDirectoryChanges)

## Stack technologique

- **Langage**: Go (1.21+)
- **Base de données**: SQLite + SQLCipher
- **Interface**: Fyne (cross-platform GUI)
- **SMB**: go-smb2
- **Sécurité**: go-keyring, crypto standard

## Statut du projet

**Version**: 1.0.0 - Application Desktop fonctionnelle ✅

### Phases de développement

- [x] Phase 0: Infrastructure (config, DB, logging)
- [x] Phase 1: Scanner de fichiers local
- [x] Phase 2: Client SMB + authentification sécurisée
- [x] Phase 3: Cache intelligent + 3-way merge
- [x] Phase 4: Moteur de synchronisation (parallel, retry, conflits)
- [x] Phase 5: Application Desktop (Fyne + system tray)
- [x] Phase 6: Watchers temps réel (local + remote)

### Prochaines étapes
- [ ] Debug Cloud Files API (ERROR_CLOUD_FILE_NOT_UNDER_SYNC_ROOT)

## Installation

### Prérequis

- Go 1.21 ou supérieur
- **MSYS2 MinGW64 GCC** (obligatoire pour CGO/Fyne)
- Git

### Build depuis les sources (Windows)

**IMPORTANT**: Utiliser MSYS2 MinGW64 GCC, pas TDM-GCC !

```bash
# Cloner le repository
git clone https://github.com/juste-un-gars/anemone_sync_windows.git
cd anemone_sync_windows

# Installer les dépendances
go mod download

# Build avec MSYS2 MinGW64
export PATH="/c/msys64/mingw64/bin:$PATH" && go build -o anemonesync.exe ./cmd/anemonesync/

# Exécuter (GUI)
./anemonesync.exe
```

### Utilisation CLI

AnemoneSync peut aussi fonctionner en ligne de commande sans interface graphique :

```bash
# Afficher l'aide
./anemonesync.exe --help

# Lister tous les jobs configurés
./anemonesync.exe --list-jobs
./anemonesync.exe -l

# Synchroniser un job spécifique par ID
./anemonesync.exe --sync 1
./anemonesync.exe -s 1

# Synchroniser tous les jobs activés
./anemonesync.exe --sync-all
./anemonesync.exe -a
```

Sans arguments, l'application démarre en mode GUI.

### Pourquoi MSYS2 MinGW64 ?

- Ce projet utilise Fyne (GUI) qui nécessite CGO
- TDM-GCC 10.3.0 produit des binaires corrompus ("n'est pas une application Win32 valide")
- MSYS2 MinGW64 GCC 15.2.0 fonctionne correctement

## Documentation

- [CLAUDE.md](CLAUDE.md) - Instructions de développement
- [ARCHITECTURE.md](ARCHITECTURE.md) - Architecture technique détaillée
- [SESSION_STATE.md](SESSION_STATE.md) - Historique des sessions de développement
- [INSTALLATION.md](INSTALLATION.md) - Guide d'installation complet
- [CONTRIBUTING.md](CONTRIBUTING.md) - Guide de contribution
- [CHANGELOG.md](CHANGELOG.md) - Historique des versions

## Structure du projet

```
AnemoneSync/
├── cmd/anemonesync/      # Point d'entrée application
├── internal/
│   ├── app/             # Application Desktop (Fyne + systray)
│   ├── sync/            # Moteur de synchronisation
│   ├── smb/             # Client SMB + credentials
│   ├── database/        # SQLite chiffrée (SQLCipher)
│   ├── scanner/         # Scanner de fichiers local
│   └── cache/           # Cache intelligent + détection changements
├── configs/             # Configurations par défaut
├── docs/                # Documentation
└── sessions/            # Sessions de développement
```

## Développement

### Tests

```bash
# Lancer tous les tests (avec MSYS2 MinGW64)
export PATH="/c/msys64/mingw64/bin:$PATH" && go test ./...

# Tests avec coverage
export PATH="/c/msys64/mingw64/bin:$PATH" && go test -cover ./...

# Tests d'un package spécifique
export PATH="/c/msys64/mingw64/bin:$PATH" && go test ./internal/sync/...
```

### Linting

```bash
golangci-lint run
```

## Contribution

Voir [CONTRIBUTING.md](CONTRIBUTING.md) pour les guidelines de contribution.

## Licence

Ce projet est sous licence [GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0).

En résumé, cela signifie que :
- ✅ Vous pouvez utiliser, modifier et distribuer ce logiciel
- ✅ Vous pouvez l'utiliser à des fins commerciales
- ⚠️ Vous devez publier le code source de toute version modifiée
- ⚠️ Si vous l'utilisez sur un serveur, vous devez rendre le code source disponible aux utilisateurs

Voir le fichier [LICENSE](LICENSE) pour les détails complets.

## Contact

**Repository**: https://github.com/juste-un-gars/anemone_sync_windows
**Mainteneur**: Franck

## Remerciements

Développé avec l'assistance de Claude (Anthropic).

---

**Dernière mise à jour**: 2026-01-27
**Version**: 1.1.0
**Status**: Application Desktop fonctionnelle + CLI
