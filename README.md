# AnemoneSync - Client de Synchronisation SMB Multi-Plateforme

[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://golang.org/)
[![Status](https://img.shields.io/badge/status-in%20development-yellow.svg)](https://github.com/juste-un-gars/anemone_sync_windows)

## Vue d'ensemble

AnemoneSync est une application de synchronisation de fichiers vers des partages SMB, fonctionnant comme OneDrive mais avec des serveurs SMB au lieu du cloud. Multi-plateforme (Windows prioritaire, puis Linux, Android, iOS).

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

**Phase actuelle**: Phase 0 - Setup et architecture

### Phases de développement

- [x] Phase 0: Setup et architecture
- [ ] Phase 1: Core - Moteur de synchronisation
- [ ] Phase 2: Sécurité et credentials
- [ ] Phase 3: Modes de synchronisation
- [ ] Phase 4: Déclenchement et planification
- [ ] Phase 5: Interface utilisateur (Windows)
- [ ] Phase 6: Performance et optimisation
- [ ] Phase 7: Réseau et mobilité
- [ ] Phase 8: Notifications et UX
- [ ] Phase 9: Internationalisation
- [ ] Phase 10: Packaging et distribution
- [ ] Phase 11+: Portage mobile et fonctionnalités avancées

## Installation

### Prérequis

- Go 1.21 ou supérieur
- GCC/MinGW (pour SQLCipher)
- Git

### Build depuis les sources

```bash
# Cloner le repository
git clone https://github.com/juste-un-gars/anemone_sync_windows.git
cd anemone_sync_windows

# Installer les dépendances
go mod download

# Build
go build -o anemone_sync cmd/smbsync/main.go

# Exécuter
./anemone_sync
```

### Build cross-platform

```bash
# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o anemone_sync.exe cmd/smbsync/main.go

# Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o anemone_sync cmd/smbsync/main.go

# macOS
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o anemone_sync cmd/smbsync/main.go
```

## Documentation

- [PROJECT.md](PROJECT.md) - Spécifications complètes du projet
- [ARCHITECTURE.md](docs/ARCHITECTURE.md) - Documentation de l'architecture (à venir)
- [USER_GUIDE.md](docs/USER_GUIDE.md) - Guide utilisateur (à venir)
- [SESSION_STATE.md](SESSION_STATE.md) - Historique des sessions de développement
- [CHANGELOG.md](CHANGELOG.md) - Historique des versions

## Structure du projet

```
anemone_sync/
├── cmd/smbsync/          # Point d'entrée
├── internal/             # Code privé
│   ├── config/          # Configuration
│   ├── credential/      # Gestion credentials
│   ├── database/        # SQLite + migrations
│   ├── sync/            # Moteur de synchronisation
│   ├── smb/             # Client SMB
│   ├── watcher/         # File system watching
│   ├── network/         # Détection réseau
│   ├── scheduler/       # Planification
│   ├── exclusion/       # Gestion exclusions
│   ├── ui/              # Interface graphique
│   ├── notification/    # Notifications
│   ├── i18n/            # Internationalisation
│   └── logger/          # Logs
├── pkg/                 # Packages réutilisables
├── configs/             # Configurations par défaut
├── build/               # Scripts de build
├── docs/                # Documentation
└── sessions/            # Archivage sessions de dev
```

## Développement

### Tests

```bash
# Lancer tous les tests
go test ./...

# Tests avec coverage
go test -cover ./...

# Tests verbeux
go test -v ./...
```

### Linting

```bash
golangci-lint run
```

## Contribution

Le projet est actuellement en développement initial. Les guidelines de contribution seront disponibles prochainement.

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

**Dernière mise à jour**: 2026-01-11
**Version**: 0.1.0-dev
**Status**: En développement actif
