package main

import (
	"fmt"
	"os"
)

const (
	appName    = "AnemoneSync"
	appVersion = "0.1.0-dev"
	appDesc    = "Client de Synchronisation SMB Multi-Plateforme"
)

func main() {
	fmt.Printf("%s v%s\n", appName, appVersion)
	fmt.Printf("%s\n\n", appDesc)

	// TODO: Phase 0
	// - Initialiser le système de configuration
	// - Initialiser la base de données SQLite + SQLCipher
	// - Tester la connexion SMB basique

	// TODO: Phase 1 - Core
	// - Implémenter le moteur de synchronisation
	// - Scanner de fichiers
	// - Calcul de hash et détection des changements

	// TODO: Phase 2 - Sécurité
	// - Intégration keystore système
	// - Gestion sécurisée des credentials

	// TODO: Phase 3+ - Fonctionnalités complètes
	// - Modes de synchronisation
	// - Interface utilisateur
	// - Planification
	// - etc.

	fmt.Println("Application en développement - Phase 0")
	fmt.Println("Voir PROJECT.md pour les spécifications complètes")

	os.Exit(0)
}
