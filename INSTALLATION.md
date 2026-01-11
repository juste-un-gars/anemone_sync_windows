# Guide d'installation - AnemoneSync

Ce guide vous accompagne dans l'installation des prérequis et le build du projet.

## Statut actuel

**Phase 0: Setup et architecture** - ✅ Structure de base créée

### Ce qui est fait

- ✅ Structure complète des dossiers
- ✅ Fichiers de base (README, LICENSE, .gitignore, CHANGELOG)
- ✅ Système d'archivage des sessions
- ✅ Configuration de base (YAML)
- ✅ Schéma de base de données SQLite
- ✅ Modules de base (config, database, logger)
- ✅ Point d'entrée main.go

### Ce qui reste à faire

- ⏳ Installer Go
- ⏳ Télécharger les dépendances Go
- ⏳ Tester la compilation
- ⏳ Initialiser Git et pousser sur GitHub

---

## Prérequis

### 1. Installation de Go

AnemoneSync nécessite Go 1.21 ou supérieur.

#### Windows

**Option A: Installateur officiel (recommandé)**

1. Téléchargez l'installateur depuis [go.dev/dl](https://go.dev/dl/)
2. Choisissez `go1.21.x.windows-amd64.msi` ou version plus récente
3. Exécutez l'installateur
4. L'installateur ajoutera automatiquement Go au PATH

**Option B: Chocolatey**

```powershell
choco install golang
```

**Option C: Scoop**

```powershell
scoop install go
```

**Vérification:**

```bash
go version
```

Devrait afficher quelque chose comme: `go version go1.21.x windows/amd64`

#### Linux

**Ubuntu/Debian:**

```bash
# Méthode officielle
wget https://go.dev/dl/go1.21.x.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.x.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

**Arch Linux:**

```bash
sudo pacman -S go
```

**Fedora:**

```bash
sudo dnf install golang
```

#### macOS

**Homebrew:**

```bash
brew install go
```

### 2. Installation de GCC (pour SQLCipher)

SQLCipher nécessite un compilateur C.

#### Windows

**Option A: TDM-GCC (recommandé, léger)**

1. Téléchargez depuis [jmeubank.github.io/tdm-gcc](https://jmeubank.github.io/tdm-gcc/)
2. Installez TDM64-GCC
3. Ajoutez `C:\TDM-GCC-64\bin` au PATH

**Option B: MSYS2 + MinGW-w64**

```bash
# Installer MSYS2 depuis https://www.msys2.org/
# Puis dans le terminal MSYS2:
pacman -S mingw-w64-x86_64-gcc
```

Ajoutez `C:\msys64\mingw64\bin` au PATH Windows

#### Linux

Généralement déjà installé. Sinon:

```bash
# Ubuntu/Debian
sudo apt install build-essential

# Fedora
sudo dnf groupinstall "Development Tools"

# Arch
sudo pacman -S base-devel
```

#### macOS

```bash
xcode-select --install
```

### 3. Git (optionnel mais recommandé)

Si pas encore installé:

**Windows:** [git-scm.com](https://git-scm.com/download/win)
**Linux:** `sudo apt install git` ou équivalent
**macOS:** `brew install git` ou inclus dans Xcode Command Line Tools

---

## Compilation du projet

### 1. Télécharger les dépendances

Une fois Go installé, dans le répertoire du projet:

```bash
cd E:\AnemoneSync
go mod download
```

Cette commande téléchargera toutes les dépendances listées dans `go.mod`.

### 2. Vérifier que tout compile

```bash
go build -o anemone_sync.exe cmd/smbsync/main.go
```

Si la compilation réussit, un fichier `anemone_sync.exe` sera créé.

### 3. Exécuter l'application

```bash
.\anemone_sync.exe
```

Pour l'instant, l'application affiche juste un message indiquant qu'elle est en développement.

---

## Configuration Git et GitHub

### 1. Initialiser le repository Git

```bash
git init
git add .
git commit -m "Initial commit - Phase 0 setup"
```

### 2. Lier au repository GitHub

```bash
git remote add origin https://github.com/juste-un-gars/anemone_sync_windows.git
git branch -M main
git push -u origin main
```

### 3. Configuration Git recommandée

```bash
# Votre nom et email
git config user.name "Votre Nom"
git config user.email "votre@email.com"

# Pour éviter les problèmes de fin de ligne Windows/Linux
git config core.autocrlf true

# Editeur par défaut (optionnel)
git config core.editor "code"  # ou "notepad", "vim", etc.
```

---

## Structure de développement recommandée

### 1. IDE / Éditeur

**Recommandations:**

- **Visual Studio Code** avec l'extension Go officielle
  - Extension: `golang.go`
  - Autocomplétion, debugging, etc.

- **GoLand** (JetBrains) - IDE complet pour Go
  - Payant mais très puissant

- **Neovim/Vim** avec gopls (Language Server Protocol)

### 2. Outils de développement utiles

```bash
# Linter (recommandé)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Formateur de code (inclus avec Go)
go fmt ./...

# Outil de vérification
go vet ./...

# Tests
go test ./...
```

### 3. Configuration VS Code recommandée

Créer `.vscode/settings.json`:

```json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "workspace",
  "editor.formatOnSave": true,
  "go.formatTool": "goimports",
  "[go]": {
    "editor.defaultFormatter": "golang.go"
  }
}
```

---

## Prochaines étapes de développement

### Phase 1: Core - Moteur de synchronisation

1. Implémenter le scanner de fichiers locaux
2. Implémenter le client SMB basique
3. Calculer les hash SHA256
4. Détecter les changements (nouveau/modifié/supprimé)
5. Transférer des fichiers (upload/download)
6. Tests unitaires

### Tests de connexion SMB

Pour tester la connexion SMB, vous aurez besoin:
- Un serveur SMB accessible (Windows Share, Samba, NAS, etc.)
- Credentials valides
- Accès réseau au serveur

Exemple de test rapide une fois le code SMB implémenté:
```bash
# Créer un fichier de test de configuration
# Puis exécuter l'application avec un test de connexion
.\anemone_sync.exe --test-smb
```

---

## Dépannage

### Erreur: "go: command not found"

Go n'est pas dans le PATH. Redémarrez votre terminal ou ajoutez manuellement Go au PATH:

**Windows:**
```
Variables d'environnement > Path > Ajouter > C:\Go\bin
```

**Linux/macOS:**
```bash
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### Erreur: "gcc: command not found" lors du build

GCC n'est pas installé ou pas dans le PATH. Voir section "Installation de GCC" ci-dessus.

### Erreur: "cannot find package"

Les dépendances ne sont pas téléchargées:
```bash
go mod download
go mod tidy
```

### Problèmes de SQLCipher sur Windows

Si vous avez des erreurs avec SQLCipher:

1. Vérifiez que GCC est bien installé: `gcc --version`
2. Installez les outils de build: `go install github.com/mattn/go-sqlite3@latest`
3. Si problème persiste, utilisez SQLite standard pour débuter (moins sécurisé mais plus simple)

---

## Support et documentation

- **Documentation projet**: [PROJECT.md](PROJECT.md)
- **Spécifications complètes**: Voir PROJECT.md section "Fonctionnalités détaillées"
- **Historique des sessions**: [SESSION_STATE.md](SESSION_STATE.md)
- **Issues GitHub**: https://github.com/juste-un-gars/anemone_sync_windows/issues

---

**Dernière mise à jour**: 2026-01-11
**Version**: Phase 0 - Setup completé
