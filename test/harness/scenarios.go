package harness

import (
	"fmt"
	"time"
)

// Scenario defines a test scenario.
type Scenario struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Job         string        `json:"job"`
	Mode        string        `json:"mode"` // mirror, upload, download
	Setup       []Action      `json:"setup,omitempty"`
	Actions     []Action      `json:"actions"`
	Expect      []Expectation `json:"expect"`
	ExpectError bool          `json:"expect_error,omitempty"` // true if sync is expected to fail
	SkipSync    bool          `json:"skip_sync,omitempty"`    // true to skip the sync step (for setup-only scenarios)
}

// GetAllScenarios returns all test scenarios.
func GetAllScenarios() []Scenario {
	scenarios := []Scenario{}

	// TEST1 - Mirror bidirectionnel
	scenarios = append(scenarios, getMirrorScenarios()...)

	// TEST2 - PC → Serveur
	scenarios = append(scenarios, getPushScenarios()...)

	// TEST3 - Serveur → PC
	scenarios = append(scenarios, getPullScenarios()...)

	// TEST4 - Conflits
	scenarios = append(scenarios, getConflictScenarios()...)

	// TEST5 - Stress
	scenarios = append(scenarios, getStressScenarios()...)

	// TEST6 - Edge cases
	scenarios = append(scenarios, getEdgeCaseScenarios()...)

	// TEST7 - Resilience reseau (tests interactifs)
	scenarios = append(scenarios, getResilienceScenarios()...)

	return scenarios
}

// getMirrorScenarios returns TEST1 scenarios (bidirectional mirror).
func getMirrorScenarios() []Scenario {
	return []Scenario{
		{
			ID:          "1.1",
			Name:        "Nouveau fichier local",
			Description: "Un fichier créé localement doit être uploadé",
			Job:         "TEST1",
			Mode:        "mirror",
			Actions: []Action{
				{Type: "create", Side: "local", Path: "new_local.txt", Content: "contenu local"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "local", Path: "new_local.txt", Expected: true},
				{Type: "file_exists", Side: "remote", Path: "new_local.txt", Expected: true},
				{Type: "files_match", Side: "both", Path: "new_local.txt"},
			},
		},
		{
			ID:          "1.2",
			Name:        "Nouveau fichier remote",
			Description: "Un fichier créé sur le serveur doit être téléchargé",
			Job:         "TEST1",
			Mode:        "mirror",
			Actions: []Action{
				{Type: "create", Side: "remote", Path: "new_remote.txt", Content: "contenu remote"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "local", Path: "new_remote.txt", Expected: true},
				{Type: "file_exists", Side: "remote", Path: "new_remote.txt", Expected: true},
				{Type: "files_match", Side: "both", Path: "new_remote.txt"},
			},
		},
		{
			ID:          "1.3",
			Name:        "Modification locale",
			Description: "Un fichier modifié localement doit être uploadé",
			Job:         "TEST1",
			Mode:        "mirror",
			Setup: []Action{
				{Type: "create", Side: "both", Path: "to_modify.txt", Content: "contenu initial"},
			},
			Actions: []Action{
				{Type: "modify", Side: "local", Path: "to_modify.txt", Content: "contenu modifié localement"},
			},
			Expect: []Expectation{
				{Type: "content_equals", Side: "local", Path: "to_modify.txt", Content: "contenu modifié localement"},
				{Type: "content_equals", Side: "remote", Path: "to_modify.txt", Content: "contenu modifié localement"},
			},
		},
		{
			ID:          "1.4",
			Name:        "Modification remote",
			Description: "Un fichier modifié sur le serveur doit être téléchargé",
			Job:         "TEST1",
			Mode:        "mirror",
			Setup: []Action{
				{Type: "create", Side: "both", Path: "to_modify.txt", Content: "contenu initial"},
			},
			Actions: []Action{
				{Type: "modify", Side: "remote", Path: "to_modify.txt", Content: "contenu modifié sur serveur"},
			},
			Expect: []Expectation{
				{Type: "content_equals", Side: "local", Path: "to_modify.txt", Content: "contenu modifié sur serveur"},
				{Type: "content_equals", Side: "remote", Path: "to_modify.txt", Content: "contenu modifié sur serveur"},
			},
		},
		{
			ID:          "1.5",
			Name:        "Suppression locale",
			Description: "Un fichier supprimé localement doit être supprimé sur le serveur",
			Job:         "TEST1",
			Mode:        "mirror",
			Setup: []Action{
				{Type: "create", Side: "both", Path: "to_delete.txt", Content: "à supprimer"},
			},
			Actions: []Action{
				{Type: "delete", Side: "local", Path: "to_delete.txt"},
			},
			Expect: []Expectation{
				{Type: "file_not_exists", Side: "local", Path: "to_delete.txt"},
				{Type: "file_not_exists", Side: "remote", Path: "to_delete.txt"},
			},
		},
		{
			ID:          "1.6",
			Name:        "Suppression remote",
			Description: "Un fichier supprimé sur le serveur doit être supprimé localement",
			Job:         "TEST1",
			Mode:        "mirror",
			Setup: []Action{
				{Type: "create", Side: "both", Path: "to_delete.txt", Content: "à supprimer"},
			},
			Actions: []Action{
				{Type: "delete", Side: "remote", Path: "to_delete.txt"},
			},
			Expect: []Expectation{
				{Type: "file_not_exists", Side: "local", Path: "to_delete.txt"},
				{Type: "file_not_exists", Side: "remote", Path: "to_delete.txt"},
			},
		},
		{
			ID:          "1.7",
			Name:        "Suppression remote + nouveau local (BUG CONNU)",
			Description: "Suppression d'un fichier remote ET création d'un nouveau local en même temps",
			Job:         "TEST1",
			Mode:        "mirror",
			Setup: []Action{
				{Type: "create", Side: "both", Path: "old_file.txt", Content: "ancien fichier"},
			},
			Actions: []Action{
				{Type: "delete", Side: "remote", Path: "old_file.txt"},
				{Type: "create", Side: "local", Path: "new_file.txt", Content: "nouveau fichier"},
			},
			Expect: []Expectation{
				{Type: "file_not_exists", Side: "both", Path: "old_file.txt"},
				{Type: "file_exists", Side: "local", Path: "new_file.txt", Expected: true},
				{Type: "file_exists", Side: "remote", Path: "new_file.txt", Expected: true},
				{Type: "files_match", Side: "both", Path: "new_file.txt"},
			},
		},
		{
			ID:          "1.8",
			Name:        "Sous-dossier local",
			Description: "Un fichier dans un sous-dossier créé localement",
			Job:         "TEST1",
			Mode:        "mirror",
			Actions: []Action{
				{Type: "create", Side: "local", Path: "subdir/nested/file.txt", Content: "fichier imbriqué"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "remote", Path: "subdir/nested/file.txt", Expected: true},
				{Type: "files_match", Side: "both", Path: "subdir/nested/file.txt"},
			},
		},
	}
}

// getPushScenarios returns TEST2 scenarios (PC → Server only).
func getPushScenarios() []Scenario {
	return []Scenario{
		{
			ID:          "2.1",
			Name:        "Nouveau local → upload",
			Description: "Un fichier créé localement doit être uploadé",
			Job:         "TEST2",
			Mode:        "upload",
			Actions: []Action{
				{Type: "create", Side: "local", Path: "push_file.txt", Content: "push content"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "remote", Path: "push_file.txt", Expected: true},
				{Type: "files_match", Side: "both", Path: "push_file.txt"},
			},
		},
		{
			ID:          "2.2",
			Name:        "Modification locale → upload",
			Description: "Un fichier modifié localement doit être uploadé",
			Job:         "TEST2",
			Mode:        "upload",
			Setup: []Action{
				{Type: "create", Side: "both", Path: "modify_push.txt", Content: "initial"},
			},
			Actions: []Action{
				{Type: "modify", Side: "local", Path: "modify_push.txt", Content: "modified locally"},
			},
			Expect: []Expectation{
				{Type: "content_equals", Side: "remote", Path: "modify_push.txt", Content: "modified locally"},
			},
		},
		{
			ID:          "2.3",
			Name:        "Suppression locale → supprime remote",
			Description: "Un fichier supprimé localement doit être supprimé sur le serveur",
			Job:         "TEST2",
			Mode:        "upload",
			Setup: []Action{
				{Type: "create", Side: "both", Path: "delete_push.txt", Content: "to delete"},
			},
			Actions: []Action{
				{Type: "delete", Side: "local", Path: "delete_push.txt"},
			},
			Expect: []Expectation{
				{Type: "file_not_exists", Side: "remote", Path: "delete_push.txt"},
			},
		},
		{
			ID:          "2.4",
			Name:        "Nouveau remote → ignoré",
			Description: "Un fichier créé sur le serveur ne doit PAS être téléchargé",
			Job:         "TEST2",
			Mode:        "upload",
			Actions: []Action{
				{Type: "create", Side: "remote", Path: "remote_only.txt", Content: "should stay remote"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "remote", Path: "remote_only.txt", Expected: true},
				{Type: "file_not_exists", Side: "local", Path: "remote_only.txt"},
			},
		},
	}
}

// getPullScenarios returns TEST3 scenarios (Server → PC only).
func getPullScenarios() []Scenario {
	return []Scenario{
		{
			ID:          "3.1",
			Name:        "Nouveau remote → download",
			Description: "Un fichier créé sur le serveur doit être téléchargé",
			Job:         "TEST3",
			Mode:        "download",
			Actions: []Action{
				{Type: "create", Side: "remote", Path: "pull_file.txt", Content: "pull content"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "local", Path: "pull_file.txt", Expected: true},
				{Type: "files_match", Side: "both", Path: "pull_file.txt"},
			},
		},
		{
			ID:          "3.2",
			Name:        "Modification remote → download",
			Description: "Un fichier modifié sur le serveur doit être téléchargé",
			Job:         "TEST3",
			Mode:        "download",
			Setup: []Action{
				{Type: "create", Side: "both", Path: "modify_pull.txt", Content: "initial"},
			},
			Actions: []Action{
				{Type: "modify", Side: "remote", Path: "modify_pull.txt", Content: "modified on server"},
			},
			Expect: []Expectation{
				{Type: "content_equals", Side: "local", Path: "modify_pull.txt", Content: "modified on server"},
			},
		},
		{
			ID:          "3.3",
			Name:        "Suppression remote → supprime local",
			Description: "Un fichier supprimé sur le serveur doit être supprimé localement",
			Job:         "TEST3",
			Mode:        "download",
			Setup: []Action{
				{Type: "create", Side: "both", Path: "delete_pull.txt", Content: "to delete"},
			},
			Actions: []Action{
				{Type: "delete", Side: "remote", Path: "delete_pull.txt"},
			},
			Expect: []Expectation{
				{Type: "file_not_exists", Side: "local", Path: "delete_pull.txt"},
			},
		},
		{
			ID:          "3.4",
			Name:        "Nouveau local → ignoré",
			Description: "Un fichier créé localement ne doit PAS être uploadé",
			Job:         "TEST3",
			Mode:        "download",
			Actions: []Action{
				{Type: "create", Side: "local", Path: "local_only.txt", Content: "should stay local"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "local", Path: "local_only.txt", Expected: true},
				{Type: "file_not_exists", Side: "remote", Path: "local_only.txt"},
			},
		},
	}
}

// getConflictScenarios returns TEST4 scenarios (conflict resolution).
func getConflictScenarios() []Scenario {
	return []Scenario{
		{
			ID:          "4.1",
			Name:        "Même fichier créé des deux côtés",
			Description: "Conflit: même nom, contenu différent → le plus récent gagne",
			Job:         "TEST4",
			Mode:        "mirror",
			Actions: []Action{
				{Type: "create", Side: "remote", Path: "conflict.txt", Content: "version serveur"},
				{Type: "create", Side: "local", Path: "conflict.txt", Content: "version locale", Delay: 100 * time.Millisecond},
			},
			Expect: []Expectation{
				// Le local est plus récent (créé après), donc devrait gagner
				{Type: "content_equals", Side: "local", Path: "conflict.txt", Content: "version locale"},
				{Type: "content_equals", Side: "remote", Path: "conflict.txt", Content: "version locale"},
			},
		},
		{
			ID:          "4.2",
			Name:        "Fichier modifié des deux côtés",
			Description: "Conflit: modifications simultanées → le plus récent gagne",
			Job:         "TEST4",
			Mode:        "mirror",
			Setup: []Action{
				{Type: "create", Side: "both", Path: "both_modified.txt", Content: "initial"},
			},
			Actions: []Action{
				{Type: "modify", Side: "remote", Path: "both_modified.txt", Content: "modifié serveur"},
				{Type: "modify", Side: "local", Path: "both_modified.txt", Content: "modifié local", Delay: 100 * time.Millisecond},
			},
			Expect: []Expectation{
				{Type: "content_equals", Side: "local", Path: "both_modified.txt", Content: "modifié local"},
				{Type: "content_equals", Side: "remote", Path: "both_modified.txt", Content: "modifié local"},
			},
		},
		{
			ID:          "4.3",
			Name:        "Modifié local + supprimé remote",
			Description: "Conflit: fichier modifié localement mais supprimé sur serveur",
			Job:         "TEST4",
			Mode:        "mirror",
			Setup: []Action{
				{Type: "create", Side: "both", Path: "mod_del.txt", Content: "initial"},
			},
			Actions: []Action{
				{Type: "delete", Side: "remote", Path: "mod_del.txt"},
				{Type: "modify", Side: "local", Path: "mod_del.txt", Content: "modifié après suppression"},
			},
			Expect: []Expectation{
				// La modification locale est plus récente, donc devrait être uploadée
				{Type: "file_exists", Side: "local", Path: "mod_del.txt", Expected: true},
				{Type: "file_exists", Side: "remote", Path: "mod_del.txt", Expected: true},
			},
		},
		{
			ID:          "4.4",
			Name:        "Supprimé local + modifié remote",
			Description: "Conflit: fichier supprimé localement mais modifié sur serveur",
			Job:         "TEST4",
			Mode:        "mirror",
			Setup: []Action{
				{Type: "create", Side: "both", Path: "del_mod.txt", Content: "initial"},
			},
			Actions: []Action{
				{Type: "delete", Side: "local", Path: "del_mod.txt"},
				{Type: "modify", Side: "remote", Path: "del_mod.txt", Content: "modifié sur serveur après suppression"},
			},
			Expect: []Expectation{
				// La modification remote est plus récente, donc devrait être téléchargée
				{Type: "file_exists", Side: "local", Path: "del_mod.txt", Expected: true},
				{Type: "file_exists", Side: "remote", Path: "del_mod.txt", Expected: true},
			},
		},
	}
}

// getStressScenarios returns TEST5 scenarios (stress/volume).
func getStressScenarios() []Scenario {
	// Generate many files scenario
	manyFilesActions := make([]Action, 50)
	manyFilesExpect := make([]Expectation, 50)
	for i := 0; i < 50; i++ {
		filename := fmt.Sprintf("file_%03d.txt", i)
		manyFilesActions[i] = Action{
			Type:    "create",
			Side:    "local",
			Path:    filename,
			Content: fmt.Sprintf("Content of file %d", i),
		}
		manyFilesExpect[i] = Expectation{
			Type:     "file_exists",
			Side:     "remote",
			Path:     filename,
			Expected: true,
		}
	}

	return []Scenario{
		{
			ID:          "5.1",
			Name:        "50 petits fichiers",
			Description: "Créer 50 petits fichiers localement",
			Job:         "TEST5",
			Mode:        "mirror",
			Actions:     manyFilesActions,
			Expect:      manyFilesExpect,
		},
		{
			ID:          "5.2",
			Name:        "Fichier moyen (1MB)",
			Description: "Créer un fichier de 1MB",
			Job:         "TEST5",
			Mode:        "mirror",
			Actions: []Action{
				{Type: "create", Side: "local", Path: "medium_file.bin", Content: GenerateContent(1024 * 1024)},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "remote", Path: "medium_file.bin", Expected: true},
				{Type: "files_match", Side: "both", Path: "medium_file.bin"},
			},
		},
		{
			ID:          "5.3",
			Name:        "Arborescence profonde",
			Description: "Créer une structure de dossiers imbriqués",
			Job:         "TEST5",
			Mode:        "mirror",
			Actions: []Action{
				{Type: "create", Side: "local", Path: "a/b/c/d/e/f/g/deep.txt", Content: "fichier profond"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "remote", Path: "a/b/c/d/e/f/g/deep.txt", Expected: true},
				{Type: "files_match", Side: "both", Path: "a/b/c/d/e/f/g/deep.txt"},
			},
		},
	}
}

// getEdgeCaseScenarios returns TEST6 scenarios (edge cases).
func getEdgeCaseScenarios() []Scenario {
	return []Scenario{
		{
			ID:          "6.1",
			Name:        "Nom avec espaces",
			Description: "Fichier avec espaces dans le nom",
			Job:         "TEST6",
			Mode:        "mirror",
			Actions: []Action{
				{Type: "create", Side: "local", Path: "mon fichier avec espaces.txt", Content: "contenu"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "remote", Path: "mon fichier avec espaces.txt", Expected: true},
				{Type: "files_match", Side: "both", Path: "mon fichier avec espaces.txt"},
			},
		},
		{
			ID:          "6.2",
			Name:        "Nom avec accents",
			Description: "Fichier avec caractères accentués",
			Job:         "TEST6",
			Mode:        "mirror",
			Actions: []Action{
				{Type: "create", Side: "local", Path: "café résumé été.txt", Content: "contenu accentué"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "remote", Path: "café résumé été.txt", Expected: true},
				{Type: "files_match", Side: "both", Path: "café résumé été.txt"},
			},
		},
		{
			ID:          "6.3",
			Name:        "Nom avec caractères spéciaux",
			Description: "Fichier avec crochets et parenthèses",
			Job:         "TEST6",
			Mode:        "mirror",
			Actions: []Action{
				{Type: "create", Side: "local", Path: "file[1](2).txt", Content: "special chars"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "remote", Path: "file[1](2).txt", Expected: true},
				{Type: "files_match", Side: "both", Path: "file[1](2).txt"},
			},
		},
		{
			ID:          "6.4",
			Name:        "Fichier vide",
			Description: "Fichier de 0 bytes",
			Job:         "TEST6",
			Mode:        "mirror",
			Actions: []Action{
				{Type: "create", Side: "local", Path: "empty.txt", Content: ""},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "remote", Path: "empty.txt", Expected: true},
			},
		},
		{
			ID:          "6.5",
			Name:        "Extension longue",
			Description: "Fichier avec extension inhabituelle",
			Job:         "TEST6",
			Mode:        "mirror",
			Actions: []Action{
				{Type: "create", Side: "local", Path: "archive.tar.gz.backup", Content: "fake archive"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "remote", Path: "archive.tar.gz.backup", Expected: true},
				{Type: "files_match", Side: "both", Path: "archive.tar.gz.backup"},
			},
		},
		{
			ID:          "6.6",
			Name:        "Dossier avec point",
			Description: "Dossier commençant par un point",
			Job:         "TEST6",
			Mode:        "mirror",
			Actions: []Action{
				{Type: "create", Side: "local", Path: ".hidden/secret.txt", Content: "hidden content"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "remote", Path: ".hidden/secret.txt", Expected: true},
				{Type: "files_match", Side: "both", Path: ".hidden/secret.txt"},
			},
		},
	}
}

// getResilienceScenarios returns TEST7 scenarios (network resilience - interactive).
// These tests require manual intervention (stopping/starting SMB server).
func getResilienceScenarios() []Scenario {
	return []Scenario{
		{
			ID:          "7.1",
			Name:        "Serveur offline au demarrage",
			Description: "Tenter un sync quand le serveur SMB est inaccessible",
			Job:         "TEST7",
			Mode:        "mirror",
			ExpectError: true, // On s'attend a ce que le sync echoue
			Setup: []Action{
				{Type: "create", Side: "local", Path: "test_offline.txt", Content: "fichier avant coupure"},
			},
			Actions: []Action{
				{Type: "wait_user", Message: "COUPER le serveur SMB maintenant, puis appuyer sur Entree"},
			},
			Expect: []Expectation{
				// Le fichier local doit toujours exister
				{Type: "file_exists", Side: "local", Path: "test_offline.txt", Expected: true},
			},
		},
		{
			ID:          "7.2",
			Name:        "Reprise apres reconnexion",
			Description: "Verifier que le sync reprend apres remise en ligne du serveur",
			Job:         "TEST7",
			Mode:        "mirror",
			SkipSync:    true, // On ne nettoie pas, on reprend l'etat precedent
			Actions: []Action{
				{Type: "wait_user", Message: "REMETTRE le serveur SMB en ligne, puis appuyer sur Entree"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "local", Path: "test_offline.txt", Expected: true},
				{Type: "file_exists", Side: "remote", Path: "test_offline.txt", Expected: true},
				{Type: "files_match", Side: "both", Path: "test_offline.txt"},
			},
		},
		{
			ID:          "7.3",
			Name:        "Gros fichier - preparation",
			Description: "Creer un fichier de 10MB pour le test d'interruption",
			Job:         "TEST7",
			Mode:        "mirror",
			Setup: []Action{
				{Type: "create", Side: "local", Path: "big_file.bin", Content: GenerateContent(10 * 1024 * 1024)},
			},
			Actions: []Action{
				{Type: "wait_user", Message: "Le fichier de 10MB est cree. Appuyer sur Entree pour sync (serveur doit etre UP)"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "remote", Path: "big_file.bin", Expected: true},
				{Type: "files_match", Side: "both", Path: "big_file.bin"},
			},
		},
		{
			ID:          "7.4",
			Name:        "Interruption pendant sync",
			Description: "Couper le serveur pendant le transfert d'un fichier volumineux",
			Job:         "TEST7",
			Mode:        "mirror",
			ExpectError: true,
			Setup: []Action{
				// Modifier le fichier pour forcer un re-upload
				{Type: "modify", Side: "local", Path: "big_file.bin", Content: GenerateContent(10 * 1024 * 1024)},
			},
			Actions: []Action{
				{Type: "wait_user", Message: "PREPAREZ-VOUS: Des que le sync demarre, COUPEZ le serveur! Appuyer sur Entree..."},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "local", Path: "big_file.bin", Expected: true},
			},
		},
		{
			ID:          "7.5",
			Name:        "Reprise apres interruption gros fichier",
			Description: "Verifier que le sync complete apres remise en ligne",
			Job:         "TEST7",
			Mode:        "mirror",
			SkipSync:    true,
			Actions: []Action{
				{Type: "wait_user", Message: "REMETTRE le serveur SMB en ligne, puis appuyer sur Entree"},
			},
			Expect: []Expectation{
				{Type: "file_exists", Side: "remote", Path: "big_file.bin", Expected: true},
				{Type: "files_match", Side: "both", Path: "big_file.bin"},
			},
		},
	}
}

// Helper for scenario formatting
func fmt_Sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
