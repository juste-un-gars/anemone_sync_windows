# Répertoire de Build

Ce répertoire contiendra les scripts et fichiers nécessaires pour compiler et packager AnemoneSync.

## Structure (à créer en Phase 10)

```
build/
├── README.md                    # Ce fichier
├── windows.sh                   # Script de build Windows
├── linux.sh                     # Script de build Linux
├── darwin.sh                    # Script de build macOS
├── installer/                   # Fichiers pour l'installeur Windows
│   ├── installer.nsi           # Script NSIS
│   ├── LICENSE.txt             # Copie de la licence
│   ├── README.txt              # Instructions
│   ├── CHANGELOG.txt           # Historique
│   └── assets/                 # Assets graphiques
│       ├── icon.ico            # Icône 256x256
│       ├── header.bmp          # En-tête 150x57
│       └── welcome.bmp         # Bienvenue 164x314
└── dist/                       # Binaires compilés (généré)
    ├── windows/
    │   ├── anemone_sync.exe
    │   └── AnemoneSync-x.x.x-Setup.exe
    ├── linux/
    │   └── anemone_sync
    └── darwin/
        └── anemone_sync
```

## Commandes de build (Phase 10)

### Windows
```bash
# Build simple
./build/windows.sh

# Build avec installeur
./build/windows.sh --installer

# Le résultat sera dans build/dist/windows/
```

### Linux
```bash
# Build
./build/linux.sh

# Build avec package .deb
./build/linux.sh --deb

# Build avec package .rpm
./build/linux.sh --rpm

# Build AppImage
./build/linux.sh --appimage
```

### macOS
```bash
# Build
./build/darwin.sh

# Build avec .dmg
./build/darwin.sh --dmg
```

## Build cross-platform depuis n'importe quel OS

```bash
# Tout compiler d'un coup
go build -o dist/windows/anemone_sync.exe cmd/smbsync/main.go
GOOS=linux GOARCH=amd64 go build -o dist/linux/anemone_sync cmd/smbsync/main.go
GOOS=darwin GOARCH=amd64 go build -o dist/darwin/anemone_sync cmd/smbsync/main.go
```

## Installeur Windows - NSIS

Documentation complète: [docs/INSTALLER.md](../docs/INSTALLER.md)

### Prérequis
- NSIS installé: https://nsis.sourceforge.io/Download
- Assets graphiques créés
- Application compilée

### Compilation de l'installeur
```bash
makensis build/installer/installer.nsi
```

### Résultat
`build/dist/windows/AnemoneSync-x.x.x-Setup.exe`

## Signature de code (optionnel mais recommandé)

Pour que Windows ne marque pas l'installeur comme "non sûr":

```bash
signtool sign /f certificate.pfx /p password /t http://timestamp.digicert.com build/dist/windows/AnemoneSync-x.x.x-Setup.exe
```

**Obtenir un certificat:**
- Commercial: Sectigo, DigiCert (~200-400€/an)
- Auto-signé: Gratuit mais Windows affichera un avertissement

## Automatisation CI/CD

GitHub Actions configuré dans `.github/workflows/release.yml` pour:
- Build automatique à chaque tag `v*`
- Création de releases GitHub
- Upload des binaires et installeurs

---

**Note**: Ce répertoire est actuellement vide car nous sommes en Phase 0.
Les scripts et fichiers seront créés en Phase 10.
