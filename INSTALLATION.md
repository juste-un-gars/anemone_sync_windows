# Guide d'installation - AnemoneSync

Ce guide vous accompagne dans l'installation des prérequis et le build du projet.

## Statut actuel

**Version 1.0.0** - Application Desktop fonctionnelle ✅

### Fonctionnalités

- ✅ Application desktop avec interface graphique (Fyne)
- ✅ System tray avec menu contextuel
- ✅ Synchronisation bidirectionnelle vers serveurs SMB
- ✅ Watchers temps réel (local + remote)
- ✅ Scheduler pour sync planifiée
- ✅ Credentials sécurisés (Windows Credential Manager)

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

### 2. Installation de GCC (OBLIGATOIRE pour CGO/Fyne)

Ce projet utilise Fyne (GUI) qui nécessite CGO et donc un compilateur C.

#### Windows - MSYS2 MinGW64 (OBLIGATOIRE)

**⚠️ ATTENTION**: N'utilisez PAS TDM-GCC ! Il produit des binaires corrompus avec ce projet.

1. Téléchargez et installez MSYS2 depuis [msys2.org](https://www.msys2.org/)
2. Ouvrez le terminal MSYS2 et installez GCC:
   ```bash
   pacman -S mingw-w64-x86_64-gcc
   ```
3. Ajoutez `C:\msys64\mingw64\bin` au PATH Windows (optionnel, sinon utilisez export)

**Vérification:**
```bash
/c/msys64/mingw64/bin/gcc --version
# Doit afficher GCC 15.x.x ou supérieur
```

#### Linux

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

### 2. Compiler l'application

**⚠️ IMPORTANT**: Toujours utiliser MSYS2 MinGW64 GCC !

```bash
# Windows (Git Bash ou MSYS2)
export PATH="/c/msys64/mingw64/bin:$PATH" && go build -o anemonesync.exe ./cmd/anemonesync/
```

Si la compilation réussit, un fichier `anemonesync.exe` (~56MB) sera créé.

**❌ Ne PAS faire:**
```bash
go build -o anemonesync.exe ./cmd/anemonesync/  # MAUVAIS - utilise TDM-GCC
```

### 3. Exécuter l'application

```bash
./anemonesync.exe
```

L'application démarre avec une icône dans le system tray. Cliquez sur l'icône pour accéder au menu (Settings, Sync Now, Quit).

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

## Utilisation

### Premier démarrage

1. Lancez `anemonesync.exe`
2. Cliquez sur l'icône dans le system tray → **Settings**
3. Dans l'onglet **SMB Connections**, ajoutez votre serveur SMB
4. Dans l'onglet **Sync Jobs**, créez un nouveau job de synchronisation
5. Configurez le dossier local, le share SMB distant, et le mode de sync

### Modes de synchronisation

- **Mirror**: Synchronisation bidirectionnelle
- **Upload only**: Local → Remote uniquement
- **Download only**: Remote → Local uniquement

### Déclenchement

- **Realtime**: Sync automatique quand des fichiers changent (local ou remote)
- **Scheduled**: Sync périodique (5m, 15m, 30m, 1h)
- **Manual**: Sync uniquement via le bouton "Sync Now"

---

## Dépannage

### Erreur: "n'est pas une application Win32 valide" ou "CETTE APPLICATION NE PEUT PAS S'EXECUTER"

**Cause**: Vous avez compilé avec TDM-GCC au lieu de MSYS2 MinGW64.

**Solution**: Recompilez avec le bon compilateur:
```bash
export PATH="/c/msys64/mingw64/bin:$PATH" && go build -o anemonesync.exe ./cmd/anemonesync/
```

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

MSYS2 MinGW64 GCC n'est pas installé. Voir section "Installation de GCC" ci-dessus.

### Erreur: "cannot find package"

Les dépendances ne sont pas téléchargées:
```bash
go mod download
go mod tidy
```

### L'icône n'apparaît pas dans le system tray

1. Vérifiez que l'application s'est bien lancée (pas d'erreur dans la console)
2. Cherchez l'icône dans les icônes cachées du system tray (flèche ^)
3. Redémarrez l'application

---

## Support et documentation

- **Instructions développement**: [CLAUDE.md](CLAUDE.md)
- **Architecture technique**: [ARCHITECTURE.md](ARCHITECTURE.md)
- **Historique des sessions**: [SESSION_STATE.md](SESSION_STATE.md)
- **Issues GitHub**: https://github.com/juste-un-gars/anemone_sync_windows/issues

---

**Dernière mise à jour**: 2026-01-18
**Version**: 1.0.0
