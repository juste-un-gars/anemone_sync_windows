# Makefile pour AnemoneSync
# Usage: make [target]

.PHONY: help build test lint fmt clean install run dev

# Variables
APP_NAME := anemone_sync
VERSION := 0.1.0-dev
BUILD_DIR := build/dist
GO := go
GOFLAGS := -v
LDFLAGS := -s -w

# Couleurs pour output
CYAN := \033[0;36m
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

## help: Affiche cette aide
help:
	@echo "$(CYAN)AnemoneSync - Makefile Help$(NC)"
	@echo ""
	@echo "$(GREEN)Targets disponibles:$(NC)"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | column -t -s ':'
	@echo ""

## install: Installe les dépendances du projet
install:
	@echo "$(CYAN)Installation des dépendances...$(NC)"
	$(GO) mod download
	$(GO) mod verify
	@echo "$(GREEN)✓ Dépendances installées$(NC)"

## build: Compile le binaire (Windows)
build:
	@echo "$(CYAN)Compilation de $(APP_NAME) (Windows)...$(NC)"
	@mkdir -p $(BUILD_DIR)/windows
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/windows/$(APP_NAME).exe cmd/smbsync/main.go
	@echo "$(GREEN)✓ Build terminé: $(BUILD_DIR)/windows/$(APP_NAME).exe$(NC)"

## build-all: Compile pour toutes les plateformes
build-all:
	@echo "$(CYAN)Compilation multi-plateforme...$(NC)"
	@mkdir -p $(BUILD_DIR)/{windows,linux,darwin}

	@echo "  → Windows amd64..."
	@GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/windows/$(APP_NAME).exe cmd/smbsync/main.go

	@echo "  → Linux amd64..."
	@GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/linux/$(APP_NAME) cmd/smbsync/main.go

	@echo "  → macOS amd64..."
	@GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/darwin/$(APP_NAME) cmd/smbsync/main.go

	@echo "$(GREEN)✓ Build multi-plateforme terminé$(NC)"

## test: Lance tous les tests
test:
	@echo "$(CYAN)Exécution des tests...$(NC)"
	$(GO) test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ Tests terminés$(NC)"

## test-coverage: Lance les tests avec rapport de couverture
test-coverage: test
	@echo "$(CYAN)Génération du rapport de couverture...$(NC)"
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Rapport généré: coverage.html$(NC)"

## bench: Lance les benchmarks
bench:
	@echo "$(CYAN)Exécution des benchmarks...$(NC)"
	$(GO) test -bench=. -benchmem ./...

## lint: Vérifie le code avec golangci-lint
lint:
	@echo "$(CYAN)Linting du code...$(NC)"
	@which golangci-lint > /dev/null || (echo "$(RED)✗ golangci-lint non installé. Installez-le: https://golangci-lint.run/usage/install/$(NC)" && exit 1)
	golangci-lint run ./...
	@echo "$(GREEN)✓ Linting terminé$(NC)"

## fmt: Formate le code
fmt:
	@echo "$(CYAN)Formatage du code...$(NC)"
	$(GO) fmt ./...
	goimports -w -local github.com/juste-un-gars/anemone_sync_windows .
	@echo "$(GREEN)✓ Code formaté$(NC)"

## vet: Analyse le code avec go vet
vet:
	@echo "$(CYAN)Analyse avec go vet...$(NC)"
	$(GO) vet ./...
	@echo "$(GREEN)✓ Analyse terminée$(NC)"

## check: Vérifie tout (fmt, vet, lint, test)
check: fmt vet lint test
	@echo "$(GREEN)✓ Toutes les vérifications passées$(NC)"

## clean: Nettoie les fichiers générés
clean:
	@echo "$(CYAN)Nettoyage...$(NC)"
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@rm -f $(APP_NAME) $(APP_NAME).exe
	@echo "$(GREEN)✓ Nettoyage terminé$(NC)"

## run: Compile et exécute l'application
run: build
	@echo "$(CYAN)Exécution de $(APP_NAME)...$(NC)"
	@./$(BUILD_DIR)/windows/$(APP_NAME).exe

## dev: Mode développement (watch and reload - nécessite air)
dev:
	@echo "$(CYAN)Mode développement...$(NC)"
	@which air > /dev/null || (echo "$(YELLOW)Installation de air...$(NC)" && go install github.com/cosmtrek/air@latest)
	air

## deps-update: Met à jour les dépendances
deps-update:
	@echo "$(CYAN)Mise à jour des dépendances...$(NC)"
	$(GO) get -u ./...
	$(GO) mod tidy
	@echo "$(GREEN)✓ Dépendances mises à jour$(NC)"

## deps-graph: Affiche le graphe des dépendances
deps-graph:
	@echo "$(CYAN)Génération du graphe des dépendances...$(NC)"
	$(GO) mod graph | modgraphviz | dot -Tpng -o deps-graph.png
	@echo "$(GREEN)✓ Graphe généré: deps-graph.png$(NC)"

## size: Affiche la taille des binaires
size:
	@echo "$(CYAN)Taille des binaires:$(NC)"
	@ls -lh $(BUILD_DIR)/windows/$(APP_NAME).exe 2>/dev/null || echo "  Windows: non compilé"
	@ls -lh $(BUILD_DIR)/linux/$(APP_NAME) 2>/dev/null || echo "  Linux: non compilé"
	@ls -lh $(BUILD_DIR)/darwin/$(APP_NAME) 2>/dev/null || echo "  macOS: non compilé"

## version: Affiche la version
version:
	@echo "$(CYAN)AnemoneSync version $(VERSION)$(NC)"

## todo: Liste tous les TODOs dans le code
todo:
	@echo "$(CYAN)TODOs dans le code:$(NC)"
	@grep -rn "TODO" internal/ cmd/ pkg/ --color=always || echo "  Aucun TODO trouvé"

## security: Vérifie les vulnérabilités de sécurité
security:
	@echo "$(CYAN)Vérification de sécurité...$(NC)"
	@which gosec > /dev/null || (echo "$(YELLOW)Installation de gosec...$(NC)" && go install github.com/securego/gosec/v2/cmd/gosec@latest)
	gosec -fmt=json -out=security-report.json ./...
	@echo "$(GREEN)✓ Rapport généré: security-report.json$(NC)"

## install-tools: Installe les outils de développement
install-tools:
	@echo "$(CYAN)Installation des outils de développement...$(NC)"
	@echo "  → golangci-lint..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "  → gosec..."
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "  → air (hot reload)..."
	@go install github.com/cosmtrek/air@latest
	@echo "$(GREEN)✓ Outils installés$(NC)"

.DEFAULT_GOAL := help
