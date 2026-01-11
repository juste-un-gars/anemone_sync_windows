# Documentation de l'Installeur Windows

## Objectif

Créer un installeur `.exe` professionnel pour AnemoneSync qui permet une installation simple en quelques clics pour les utilisateurs finaux.

---

## Choix de la technologie d'installeur

### Options disponibles

#### 1. NSIS (Nullsoft Scriptable Install System) ⭐ RECOMMANDÉ

**Avantages:**
- Gratuit et open source
- Très populaire et bien documenté
- Produit des installeurs compacts
- Support complet Windows (XP à 11)
- Nombreux plugins disponibles
- Interface personnalisable

**Inconvénients:**
- Langage de script propriétaire
- Courbe d'apprentissage modérée

**Cas d'usage:** VLC, 7-Zip, OBS Studio

#### 2. Inno Setup

**Avantages:**
- Gratuit et open source
- Langage de script type Pascal (plus lisible)
- Très flexible
- Support Unicode complet

**Inconvénients:**
- Moins populaire que NSIS
- Fichiers installeurs légèrement plus gros

#### 3. WiX Toolset (Windows Installer XML)

**Avantages:**
- Standard Microsoft
- Produit des fichiers .msi
- Très professionnel
- Intégration parfaite avec l'écosystème Windows

**Inconvénients:**
- Complexe à apprendre (XML verbeux)
- Nécessite .NET Framework
- Courbe d'apprentissage élevée

#### 4. Advanced Installer

**Avantages:**
- Interface graphique complète
- Très professionnel
- Support MSI et EXE

**Inconvénients:**
- Payant (version gratuite limitée)
- Overkill pour petits projets

**Recommandation finale: NSIS**
- Bon compromis simplicité/puissance
- Largement utilisé dans l'open source
- Documentation excellente
- Gratuit et sans limitation

---

## Fonctionnalités requises de l'installeur

### 1. Installation basique

- [x] Sélection du répertoire d'installation (défaut: `C:\Program Files\AnemoneSync`)
- [x] Copie des fichiers binaires
- [x] Création des raccourcis (Menu Démarrer, Bureau)
- [x] Désinstalleur automatique

### 2. Détection des dépendances

- [x] Vérifier si GCC/MinGW est installé (pour SQLCipher)
- [x] Option pour télécharger/installer automatiquement si manquant
- [x] Message clair si des dépendances manquent

### 3. Configuration initiale

- [x] Créer les répertoires de données:
  - `%APPDATA%\AnemoneSync\` (config, DB, logs)
- [x] Copier les fichiers de configuration par défaut
- [x] Initialiser la base de données vide

### 4. Démarrage automatique

- [x] Option: "Démarrer AnemoneSync au démarrage de Windows"
- [x] Ajouter dans le registre: `HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run`

### 5. Désinstallation propre

- [x] Supprimer les fichiers du programme
- [x] Option: "Conserver mes données et ma configuration" (coché par défaut)
- [x] Nettoyer les raccourcis
- [x] Nettoyer le registre
- [x] Supprimer l'entrée de démarrage automatique si présente

### 6. Mise à jour

- [x] Détecter si une version précédente est installée
- [x] Proposer de désinstaller l'ancienne version
- [x] Préserver les données utilisateur
- [x] Migration de configuration si nécessaire

### 7. Interface utilisateur

- [x] Logo et icône AnemoneSync
- [x] Écran de bienvenue
- [x] Licence AGPL-3.0 à accepter
- [x] Sélection du répertoire
- [x] Options d'installation (raccourcis, démarrage auto)
- [x] Barre de progression
- [x] Écran de fin avec option "Lancer AnemoneSync"

### 8. Sécurité

- [x] Signature numérique du binaire (code signing)
- [x] Vérification de l'intégrité (checksum)
- [x] Élévation de privilèges (UAC) si nécessaire

---

## Structure des fichiers à installer

```
C:\Program Files\AnemoneSync\
├── anemone_sync.exe           # Binaire principal
├── LICENSE.txt                # Licence AGPL-3.0
├── README.txt                 # Instructions de base
├── CHANGELOG.txt              # Historique des versions
├── configs\
│   ├── default_config.yaml    # Config par défaut
│   └── default_exclusions.json
└── uninstall.exe              # Désinstalleur

%APPDATA%\AnemoneSync\
├── config.yaml                # Config utilisateur (copie modifiable)
├── anemone_sync.db           # Base de données (créée au 1er lancement)
└── logs\                      # Logs de l'application
```

---

## Script NSIS - Structure de base

```nsis
; AnemoneSync Installer
; Version: 0.1.0

;--------------------------------
; Includes

!include "MUI2.nsh"
!include "FileFunc.nsh"
!include "LogicLib.nsh"

;--------------------------------
; Configuration

!define APP_NAME "AnemoneSync"
!define APP_VERSION "0.1.0"
!define APP_PUBLISHER "Franck"
!define APP_URL "https://github.com/juste-un-gars/anemone_sync_windows"
!define APP_EXE "anemone_sync.exe"

Name "${APP_NAME} ${APP_VERSION}"
OutFile "AnemoneSync-${APP_VERSION}-Setup.exe"
InstallDir "$PROGRAMFILES64\${APP_NAME}"
InstallDirRegKey HKLM "Software\${APP_NAME}" "InstallDir"

RequestExecutionLevel admin

;--------------------------------
; Interface Settings

!define MUI_ICON "assets\icon.ico"
!define MUI_UNICON "assets\icon.ico"
!define MUI_HEADERIMAGE
!define MUI_HEADERIMAGE_BITMAP "assets\header.bmp"
!define MUI_WELCOMEFINISHPAGE_BITMAP "assets\welcome.bmp"
!define MUI_ABORTWARNING

;--------------------------------
; Pages

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "LICENSE.txt"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_UNPAGE_FINISH

;--------------------------------
; Languages

!insertmacro MUI_LANGUAGE "French"
!insertmacro MUI_LANGUAGE "English"

;--------------------------------
; Installation Section

Section "Programme principal" SecMain
  SectionIn RO  ; Read-only, toujours installé

  SetOutPath "$INSTDIR"

  ; Fichiers principaux
  File "build\${APP_EXE}"
  File "LICENSE.txt"
  File "README.txt"
  File "CHANGELOG.txt"

  ; Configurations
  SetOutPath "$INSTDIR\configs"
  File "configs\default_config.yaml"
  File "configs\default_exclusions.json"

  ; Créer les répertoires utilisateur
  SetOutPath "$APPDATA\${APP_NAME}"
  File /oname=config.yaml "configs\default_config.yaml"
  CreateDirectory "$APPDATA\${APP_NAME}\logs"

  ; Écrire dans le registre
  WriteRegStr HKLM "Software\${APP_NAME}" "InstallDir" "$INSTDIR"
  WriteRegStr HKLM "Software\${APP_NAME}" "Version" "${APP_VERSION}"

  ; Créer le désinstalleur
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Ajouter dans Ajout/Suppression de programmes
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
                   "DisplayName" "${APP_NAME}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
                   "DisplayVersion" "${APP_VERSION}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
                   "Publisher" "${APP_PUBLISHER}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
                   "URLInfoAbout" "${APP_URL}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
                   "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
                     "NoModify" 1
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
                     "NoRepair" 1

SectionEnd

Section "Raccourcis Menu Démarrer" SecStartMenu
  CreateDirectory "$SMPROGRAMS\${APP_NAME}"
  CreateShortCut "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"
  CreateShortCut "$SMPROGRAMS\${APP_NAME}\Désinstaller.lnk" "$INSTDIR\uninstall.exe"
SectionEnd

Section "Raccourci Bureau" SecDesktop
  CreateShortCut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"
SectionEnd

Section "Démarrage automatique" SecAutoStart
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" \
                   "${APP_NAME}" "$INSTDIR\${APP_EXE}"
SectionEnd

;--------------------------------
; Descriptions

LangString DESC_SecMain ${LANG_FRENCH} "Fichiers principaux d'AnemoneSync (requis)"
LangString DESC_SecStartMenu ${LANG_FRENCH} "Créer des raccourcis dans le Menu Démarrer"
LangString DESC_SecDesktop ${LANG_FRENCH} "Créer un raccourci sur le Bureau"
LangString DESC_SecAutoStart ${LANG_FRENCH} "Lancer AnemoneSync automatiquement au démarrage de Windows"

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecMain} $(DESC_SecMain)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecStartMenu} $(DESC_SecStartMenu)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} $(DESC_SecDesktop)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecAutoStart} $(DESC_SecAutoStart)
!insertmacro MUI_FUNCTION_DESCRIPTION_END

;--------------------------------
; Uninstaller Section

Section "Uninstall"

  ; Supprimer les fichiers
  Delete "$INSTDIR\${APP_EXE}"
  Delete "$INSTDIR\LICENSE.txt"
  Delete "$INSTDIR\README.txt"
  Delete "$INSTDIR\CHANGELOG.txt"
  Delete "$INSTDIR\uninstall.exe"

  RMDir /r "$INSTDIR\configs"
  RMDir "$INSTDIR"

  ; Supprimer les raccourcis
  Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk"
  Delete "$SMPROGRAMS\${APP_NAME}\Désinstaller.lnk"
  RMDir "$SMPROGRAMS\${APP_NAME}"
  Delete "$DESKTOP\${APP_NAME}.lnk"

  ; Supprimer du démarrage automatique
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "${APP_NAME}"

  ; Option: conserver les données utilisateur
  MessageBox MB_YESNO|MB_ICONQUESTION \
    "Voulez-vous conserver vos données et votre configuration ?$\n\
     (Base de données, configuration personnalisée, logs)" \
    /SD IDYES IDYES keep_data

  ; Supprimer les données
  RMDir /r "$APPDATA\${APP_NAME}"

  keep_data:

  ; Nettoyer le registre
  DeleteRegKey HKLM "Software\${APP_NAME}"
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}"

SectionEnd
```

---

## Build du package final

### 1. Préparer les assets

Créer le dossier `build/installer/` avec:
- `assets/icon.ico` - Icône de l'application (256x256)
- `assets/header.bmp` - Image d'en-tête (150x57)
- `assets/welcome.bmp` - Image de bienvenue (164x314)

### 2. Compiler l'installeur

```bash
# Après avoir compilé l'application Go
go build -ldflags="-s -w -H=windowsgui" -o build/anemone_sync.exe cmd/smbsync/main.go

# Copier les fichiers nécessaires
cp LICENSE build/installer/LICENSE.txt
cp README.md build/installer/README.txt
cp CHANGELOG.md build/installer/CHANGELOG.txt
cp -r configs build/installer/

# Compiler l'installeur NSIS
makensis build/installer/installer.nsi

# Résultat: build/installer/AnemoneSync-0.1.0-Setup.exe
```

### 3. Signature du code (optionnel mais recommandé)

Pour que Windows ne marque pas l'installeur comme "non sûr", il faut un certificat de signature de code:

```bash
# Avec SignTool (Windows SDK)
signtool sign /f certificate.pfx /p password /t http://timestamp.digicert.com build/installer/AnemoneSync-0.1.0-Setup.exe
```

**Obtenir un certificat:**
- Certificat commercial: Sectigo, DigiCert (~200-400€/an)
- Certificat gratuit: Très difficile pour applications Windows
- Alternative: Auto-signé (Windows affichera un avertissement)

---

## Automatisation avec GitHub Actions

Créer `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build-windows:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Build
        run: |
          go build -ldflags="-s -w -H=windowsgui" -o build/anemone_sync.exe cmd/smbsync/main.go

      - name: Install NSIS
        run: choco install nsis -y

      - name: Prepare installer files
        run: |
          cp LICENSE build/installer/LICENSE.txt
          cp README.md build/installer/README.txt
          cp CHANGELOG.md build/installer/CHANGELOG.txt
          cp -r configs build/installer/

      - name: Build installer
        run: makensis build/installer/installer.nsi

      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: build/installer/AnemoneSync-*.exe
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

## Phase d'implémentation

Cette fonctionnalité sera développée en **Phase 10: Packaging et distribution**

### Étapes:

1. ✅ Phase 0-9: Développer l'application complète
2. **Phase 10.1**: Créer les assets graphiques (icônes, images)
3. **Phase 10.2**: Écrire le script NSIS complet
4. **Phase 10.3**: Tester l'installeur sur différentes versions de Windows
5. **Phase 10.4**: Obtenir un certificat de signature de code (optionnel)
6. **Phase 10.5**: Automatiser le build avec GitHub Actions
7. **Phase 10.6**: Publier la première release

---

## Tests de l'installeur

### Checklist de test

- [ ] Installation fraîche sur Windows 10
- [ ] Installation fraîche sur Windows 11
- [ ] Mise à jour depuis version précédente
- [ ] Installation sans droits admin (doit demander)
- [ ] Installation avec droits admin
- [ ] Désinstallation avec conservation des données
- [ ] Désinstallation complète
- [ ] Vérifier que les raccourcis fonctionnent
- [ ] Vérifier le démarrage automatique
- [ ] Vérifier dans Ajout/Suppression de programmes
- [ ] Test sur machine virtuelle propre

---

## Alternatives plus simples (court terme)

En attendant la Phase 10, on peut distribuer:

### Option 1: Archive ZIP
- Binaire compilé + README
- L'utilisateur extrait et lance
- Pas d'installation propre

### Option 2: Portable
- Version portable qui s'exécute depuis n'importe où
- Stocke tout dans son propre dossier
- Pas d'écriture dans Program Files ou AppData

### Option 3: Script PowerShell d'installation
- Script simple qui copie les fichiers
- Crée les raccourcis
- Plus facile à maintenir qu'un vrai installeur

---

**Dernière mise à jour**: 2026-01-11
**Statut**: Documentation - À implémenter en Phase 10
