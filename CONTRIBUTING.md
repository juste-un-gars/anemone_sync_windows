# Guide de Contribution - AnemoneSync

Merci de votre intérêt pour contribuer à AnemoneSync! 🎉

## Table des matières

- [Code de conduite](#code-de-conduite)
- [Comment contribuer](#comment-contribuer)
- [Standards de code](#standards-de-code)
- [Processus de développement](#processus-de-développement)
- [Soumettre une Pull Request](#soumettre-une-pull-request)
- [Signaler des bugs](#signaler-des-bugs)
- [Proposer des fonctionnalités](#proposer-des-fonctionnalités)

---

## Code de conduite

Ce projet adhère au [Code de Conduite](CODE_OF_CONDUCT.md). En participant, vous vous engagez à respecter ces termes.

---

## Comment contribuer

### Types de contributions acceptées

- 🐛 **Corrections de bugs**
- ✨ **Nouvelles fonctionnalités** (discutez d'abord via une issue)
- 📝 **Amélioration de la documentation**
- 🌍 **Traductions** (i18n)
- 🧪 **Tests** (toujours les bienvenus!)
- 🎨 **Design et UX**

### Avant de commencer

1. **Vérifiez les issues existantes** pour éviter les doublons
2. **Créez une issue** pour discuter des changements importants
3. **Lisez la documentation** : PROJECT.md, INSTALLATION.md, docs/

---

## Standards de code

### Go

- **Version**: Go 1.21+
- **Style**: Suivre les conventions Go standards (`go fmt`, `go vet`)
- **Linting**: Utiliser `golangci-lint`
- **Imports**: Groupés et ordonnés (stdlib, external, internal)

```go
// Bon exemple
import (
    "fmt"
    "os"

    "github.com/spf13/viper"
    "go.uber.org/zap"

    "github.com/juste-un-gars/anemone_sync_windows/internal/config"
)
```

### Nommage

- **Packages**: lowercase, noms courts et descriptifs
- **Fonctions exportées**: PascalCase
- **Fonctions privées**: camelCase
- **Constantes**: PascalCase ou SCREAMING_SNAKE_CASE pour valeurs globales

### Commentaires

- **Fonctions exportées**: Doivent avoir un commentaire doc
- **Code complexe**: Expliquer le "pourquoi", pas le "comment"
- **TODOs**: Format `// TODO(username): description`

```go
// LoadConfig charge la configuration depuis le fichier spécifié ou utilise
// les valeurs par défaut si le fichier n'existe pas.
func LoadConfig(path string) (*Config, error) {
    // ...
}
```

### Tests

- **Couverture**: Minimum 70% pour le nouveau code
- **Nommage**: `TestFunctionName_Scenario`
- **Table-driven tests**: Préféré pour tests multiples

```go
func TestLoadConfig_WithValidFile(t *testing.T) {
    // ...
}
```

### Sécurité

**CRITIQUE**: Ce projet gère des credentials sensibles

- ❌ **JAMAIS** de mots de passe en clair
- ❌ **JAMAIS** de credentials dans les logs
- ❌ **JAMAIS** de secrets dans le code ou les commits
- ✅ Toujours utiliser les keystores système
- ✅ Zérotiser la mémoire après usage de données sensibles
- ✅ Valider et sanitiser toutes les entrées utilisateur

---

## Processus de développement

### 1. Fork et Clone

```bash
# Fork sur GitHub, puis:
git clone https://github.com/VOTRE-USERNAME/anemone_sync_windows.git
cd anemone_sync_windows
git remote add upstream https://github.com/juste-un-gars/anemone_sync_windows.git
```

### 2. Créer une branche

```bash
git checkout -b feature/ma-fonctionnalite
# ou
git checkout -b fix/correction-bug
```

**Convention de nommage des branches**:
- `feature/description` - Nouvelles fonctionnalités
- `fix/description` - Corrections de bugs
- `docs/description` - Documentation
- `refactor/description` - Refactoring
- `test/description` - Ajout de tests

### 3. Développer

**⚠️ IMPORTANT**: Toujours utiliser MSYS2 MinGW64 GCC pour la compilation !

```bash
# Installer les dépendances
go mod download

# Lancer les tests
export PATH="/c/msys64/mingw64/bin:$PATH" && go test ./...

# Vérifier le formatage
go fmt ./...

# Linter
golangci-lint run

# Build (Windows)
export PATH="/c/msys64/mingw64/bin:$PATH" && go build -o anemonesync.exe ./cmd/anemonesync/
```

Voir [CLAUDE.md](CLAUDE.md) pour plus de détails sur l'environnement de développement.

### 4. Commiter

**Format des messages de commit**:

```
type(scope): Brief summary

Details if needed.

Co-Authored-By: Claude <noreply@anthropic.com>
```

**Types de commit**:
- `feat` - Nouvelle fonctionnalité
- `fix` - Correction de bug
- `docs` - Documentation
- `refactor` - Refactoring
- `test` - Ajout/modification de tests
- `chore` - Maintenance (deps, build, etc.)

**Scopes courants**: `sync`, `smb`, `app`, `db`, `scanner`, `cache`

**Exemples**:
```
feat(sync): Add stop button for running sync

Implement cancel functionality with context cancellation
for all sync operations.

Co-Authored-By: Claude <noreply@anthropic.com>
```

```
fix(smb): Handle connection timeout gracefully

Add retry logic with exponential backoff for transient
network errors.

Co-Authored-By: Claude <noreply@anthropic.com>
```

```
docs: Update SESSION_STATE.md - Session 021 complete
```

### 5. Pousser et créer une PR

```bash
git push origin feature/ma-fonctionnalite
```

Puis créez une Pull Request sur GitHub.

---

## Soumettre une Pull Request

### Checklist avant soumission

- [ ] Le code compile sans erreur
- [ ] Tous les tests passent (`go test ./...`)
- [ ] Le linter ne signale aucune erreur (`golangci-lint run`)
- [ ] Le code est formaté (`go fmt ./...`)
- [ ] Les nouveaux fichiers ont des tests
- [ ] La documentation est à jour
- [ ] Le CHANGELOG.md est mis à jour (si applicable)
- [ ] Les commits sont propres et bien formatés
- [ ] Aucun secret ou credential dans le code

### Description de la PR

Utilisez le template suivant:

```markdown
## Description
Brève description des changements.

## Type de changement
- [ ] Bug fix (non-breaking change)
- [ ] New feature (non-breaking change)
- [ ] Breaking change
- [ ] Documentation update

## Comment tester
1. Étapes pour reproduire/tester

## Checklist
- [ ] Tests ajoutés/mis à jour
- [ ] Documentation mise à jour
- [ ] CHANGELOG.md mis à jour
```

### Review process

1. Un mainteneur reviewera votre PR
2. Des changements peuvent être demandés
3. Une fois approuvée, la PR sera mergée
4. La branche sera automatiquement supprimée

---

## Signaler des bugs

### Avant de signaler

1. **Vérifiez les issues existantes**
2. **Vérifiez que vous utilisez la dernière version**
3. **Essayez de reproduire avec une configuration minimale**

### Template de bug report

```markdown
**Description du bug**
Description claire et concise.

**Comment reproduire**
1. Aller à '...'
2. Cliquer sur '...'
3. Voir l'erreur

**Comportement attendu**
Ce qui devrait se passer.

**Screenshots**
Si applicable.

**Environnement**
- OS: [e.g. Windows 11]
- Version d'AnemoneSync: [e.g. 0.2.0]
- Version de Go: [e.g. 1.21.5]

**Logs**
```
Coller les logs pertinents ici
```

**Informations additionnelles**
Tout autre contexte utile.
```

---

## Proposer des fonctionnalités

### Template de feature request

```markdown
**Le problème**
Description du problème que cette fonctionnalité résoudrait.

**La solution proposée**
Comment vous imaginez cette fonctionnalité.

**Alternatives considérées**
Autres solutions envisagées.

**Contexte additionnel**
Tout autre contexte, screenshots, exemples.
```

### Discussion

Les features importantes doivent être discutées avant implémentation:
1. Créez une issue avec le label `enhancement`
2. Attendez les retours des mainteneurs
3. Une fois validée, vous pouvez commencer l'implémentation

---

## Structure du projet

Voir [ARCHITECTURE.md](ARCHITECTURE.md) pour l'architecture complète.

```
AnemoneSync/
├── cmd/anemonesync/  # Point d'entrée application
├── internal/
│   ├── app/         # Application Desktop (Fyne + systray)
│   ├── sync/        # Moteur de synchronisation
│   ├── smb/         # Client SMB + credentials
│   ├── database/    # SQLite chiffrée
│   ├── scanner/     # Scanner de fichiers
│   └── cache/       # Cache intelligent
├── configs/         # Configurations par défaut
├── docs/            # Documentation
└── sessions/        # Sessions de développement
```

---

## Ressources

- [CLAUDE.md](CLAUDE.md) - Instructions de développement
- [ARCHITECTURE.md](ARCHITECTURE.md) - Architecture technique
- [INSTALLATION.md](INSTALLATION.md) - Guide d'installation
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://golang.org/doc/effective_go.html)

---

## Licence

En contribuant à AnemoneSync, vous acceptez que vos contributions soient sous licence [AGPL-3.0](LICENSE).

---

## Questions?

- 💬 Ouvrez une issue de type "question"
- 📧 Contactez les mainteneurs
- 📖 Consultez la documentation

---

**Merci pour votre contribution! 🙏**

Chaque contribution, aussi petite soit-elle, est précieuse pour le projet.
