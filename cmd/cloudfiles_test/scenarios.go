//go:build windows
// +build windows

package main

import "context"

// Scenario defines a test scenario.
type Scenario struct {
	ID          string
	Name        string
	Description string
	Run         func(ctx context.Context, r *Runner) error
}

// TestResult holds the result of a test.
type TestResult struct {
	ID       string
	Name     string
	Passed   bool
	Error    string
	Duration int64 // milliseconds
}

// GetAllScenarios returns all available test scenarios.
func GetAllScenarios() []Scenario {
	return []Scenario{
		{
			ID:          "T1",
			Name:        "Hydratation basique",
			Description: "Fichier distant -> placeholder -> lire -> hydraté",
			Run:         runT1BasicHydration,
		},
		{
			ID:          "T2",
			Name:        "Déshydratation basique",
			Description: "Fichier hydraté -> DehydrateFile() -> PARTIAL",
			Run:         runT2BasicDehydration,
		},
		{
			ID:          "T3",
			Name:        "Cycle complet",
			Description: "Hydrate -> Modify -> Sync -> Dehydrate",
			Run:         runT3FullCycle,
		},
		{
			ID:          "T4",
			Name:        "Upload local",
			Description: "Créer fichier local -> Sync -> Upload serveur",
			Run:         runT4LocalUpload,
		},
		{
			ID:          "T5",
			Name:        "Structure imbriquée",
			Description: "Dossier 1/A/B/ avec sous-fichiers",
			Run:         runT5NestedStructure,
		},
		{
			ID:          "T6",
			Name:        "Gros fichier",
			Description: "345MB exe, vérifier progress",
			Run:         runT6LargeFile,
		},
		{
			ID:          "T7",
			Name:        "Modif serveur",
			Description: "Modifier fichier distant -> Sync -> MAJ placeholder",
			Run:         runT7RemoteModify,
		},
		{
			ID:          "T8",
			Name:        "Suppression serveur",
			Description: "Supprimer distant -> Sync -> Suppression locale",
			Run:         runT8RemoteDelete,
		},
	}
}

// GetScenarioByID returns a scenario by its ID.
func GetScenarioByID(id string) *Scenario {
	for _, s := range GetAllScenarios() {
		if s.ID == id {
			return &s
		}
	}
	return nil
}

// T1: Basic Hydration
// 1. Create file on remote
// 2. Sync to create placeholder
// 3. Read placeholder (triggers hydration)
// 4. Verify file is hydrated (not PARTIAL)
func runT1BasicHydration(ctx context.Context, r *Runner) error {
	const testFile = "t1_hydration_test.txt"
	const content = "Test content for hydration T1"

	r.Log("Étape 1: Création fichier distant")
	if err := r.CreateRemoteFile(testFile, []byte(content)); err != nil {
		return err
	}

	r.Log("Étape 2: Sync pour créer placeholder")
	if err := r.RunSync(ctx); err != nil {
		return err
	}

	r.Log("Étape 3: Vérifier placeholder existe")
	if err := r.VerifyPlaceholderExists(testFile); err != nil {
		return err
	}

	r.Log("Étape 4: Lire fichier (trigger hydration)")
	data, err := r.ReadLocalFile(testFile)
	if err != nil {
		return err
	}

	r.Log("Étape 5: Vérifier contenu")
	if string(data) != content {
		return r.Errorf("contenu incorrect: got %q, want %q", string(data), content)
	}

	r.Log("Étape 6: Vérifier fichier hydraté")
	if err := r.VerifyFileHydrated(testFile); err != nil {
		return err
	}

	return nil
}

// T2: Basic Dehydration
// 1. Create file on remote
// 2. Sync and hydrate
// 3. Dehydrate file
// 4. Verify file is PARTIAL (dehydrated)
func runT2BasicDehydration(ctx context.Context, r *Runner) error {
	const testFile = "t2_dehydration_test.txt"
	const content = "Test content for dehydration T2"

	r.Log("Étape 1: Création fichier distant")
	if err := r.CreateRemoteFile(testFile, []byte(content)); err != nil {
		return err
	}

	r.Log("Étape 2: Sync pour créer placeholder")
	if err := r.RunSync(ctx); err != nil {
		return err
	}

	r.Log("Étape 3: Hydrater le fichier")
	if _, err := r.ReadLocalFile(testFile); err != nil {
		return err
	}

	r.Log("Étape 4: Vérifier fichier hydraté")
	if err := r.VerifyFileHydrated(testFile); err != nil {
		return err
	}

	r.Log("Étape 5: Déshydrater le fichier")
	if err := r.DehydrateFile(testFile); err != nil {
		return err
	}

	r.Log("Étape 6: Vérifier fichier déshydraté")
	if err := r.VerifyFileDehydrated(testFile); err != nil {
		return err
	}

	return nil
}

// T3: Full Cycle
// 1. Create remote file, sync, hydrate
// 2. Modify local file
// 3. Sync (upload changes)
// 4. Dehydrate
// 5. Verify remote has new content
func runT3FullCycle(ctx context.Context, r *Runner) error {
	const testFile = "t3_cycle_test.txt"
	const content1 = "Original content T3"
	const content2 = "Modified content T3"

	r.Log("Étape 1: Création fichier distant")
	if err := r.CreateRemoteFile(testFile, []byte(content1)); err != nil {
		return err
	}

	r.Log("Étape 2: Sync et hydratation")
	if err := r.RunSync(ctx); err != nil {
		return err
	}
	if _, err := r.ReadLocalFile(testFile); err != nil {
		return err
	}

	r.Log("Étape 3: Modifier fichier local")
	if err := r.WriteLocalFile(testFile, []byte(content2)); err != nil {
		return err
	}

	r.Log("Étape 4: Sync (upload)")
	if err := r.RunSync(ctx); err != nil {
		return err
	}

	r.Log("Étape 5: Vérifier contenu distant")
	remoteData, err := r.ReadRemoteFile(testFile)
	if err != nil {
		return err
	}
	if string(remoteData) != content2 {
		return r.Errorf("contenu distant incorrect: got %q, want %q", string(remoteData), content2)
	}

	r.Log("Étape 6: Déshydrater")
	if err := r.DehydrateFile(testFile); err != nil {
		return err
	}

	r.Log("Étape 7: Vérifier fichier déshydraté")
	return r.VerifyFileDehydrated(testFile)
}

// T4: Local Upload
// 1. Create local file directly
// 2. Sync (upload to server)
// 3. Verify file exists on remote
func runT4LocalUpload(ctx context.Context, r *Runner) error {
	const testFile = "t4_upload_test.txt"
	const content = "New local file T4"

	r.Log("Étape 1: Créer fichier local")
	if err := r.CreateLocalFile(testFile, []byte(content)); err != nil {
		return err
	}

	r.Log("Étape 2: Sync (upload)")
	if err := r.RunSync(ctx); err != nil {
		return err
	}

	r.Log("Étape 3: Vérifier fichier distant")
	remoteData, err := r.ReadRemoteFile(testFile)
	if err != nil {
		return err
	}
	if string(remoteData) != content {
		return r.Errorf("contenu distant incorrect: got %q, want %q", string(remoteData), content)
	}

	return nil
}

// T5: Nested Structure
// 1. Create nested folder structure on remote
// 2. Sync to create placeholders
// 3. Verify all placeholders exist
// 4. Hydrate one file deep in structure
func runT5NestedStructure(ctx context.Context, r *Runner) error {
	files := map[string]string{
		"t5/level1.txt":           "Level 1",
		"t5/A/level2.txt":         "Level 2",
		"t5/A/B/level3.txt":       "Level 3",
		"t5/A/B/C/level4.txt":     "Level 4",
	}

	r.Log("Étape 1: Créer structure sur le serveur")
	for path, content := range files {
		if err := r.CreateRemoteFile(path, []byte(content)); err != nil {
			return err
		}
	}

	r.Log("Étape 2: Sync pour créer placeholders")
	if err := r.RunSync(ctx); err != nil {
		return err
	}

	r.Log("Étape 3: Vérifier tous les placeholders")
	for path := range files {
		if err := r.VerifyPlaceholderExists(path); err != nil {
			return err
		}
	}

	r.Log("Étape 4: Hydrater fichier niveau 4")
	data, err := r.ReadLocalFile("t5/A/B/C/level4.txt")
	if err != nil {
		return err
	}
	if string(data) != "Level 4" {
		return r.Errorf("contenu incorrect: got %q, want %q", string(data), "Level 4")
	}

	return nil
}

// T6: Large File
// 1. Create large file on remote (or use existing)
// 2. Sync to create placeholder
// 3. Hydrate and verify progress
// 4. Dehydrate
func runT6LargeFile(ctx context.Context, r *Runner) error {
	r.Log("Étape 1: Vérifier fichier source volumineux")
	sourceFile, size, err := r.FindLargeSourceFile(50 * 1024 * 1024) // 50MB min
	if err != nil {
		return r.Errorf("pas de fichier source volumineux trouvé: %v", err)
	}
	r.Logf("  Trouvé: %s (%.2f MB)", sourceFile, float64(size)/(1024*1024))

	const testFile = "t6_large_file.bin"

	r.Log("Étape 2: Copier vers le serveur")
	if err := r.CopyFileToRemote(sourceFile, testFile); err != nil {
		return err
	}

	r.Log("Étape 3: Sync pour créer placeholder")
	if err := r.RunSync(ctx); err != nil {
		return err
	}

	r.Log("Étape 4: Vérifier placeholder")
	if err := r.VerifyPlaceholderExists(testFile); err != nil {
		return err
	}

	r.Log("Étape 5: Hydrater (avec progress)")
	if err := r.HydrateFileWithProgress(ctx, testFile); err != nil {
		return err
	}

	r.Log("Étape 6: Vérifier fichier hydraté")
	if err := r.VerifyFileHydrated(testFile); err != nil {
		return err
	}

	r.Log("Étape 7: Déshydrater")
	if err := r.DehydrateFile(testFile); err != nil {
		return err
	}

	return r.VerifyFileDehydrated(testFile)
}

// T7: Remote Modify
// 1. Create file on remote, sync, hydrate
// 2. Modify file on remote
// 3. Sync
// 4. Verify local has new content
func runT7RemoteModify(ctx context.Context, r *Runner) error {
	const testFile = "t7_remote_modify.txt"
	const content1 = "Original T7"
	const content2 = "Modified on server T7"

	r.Log("Étape 1: Créer fichier distant")
	if err := r.CreateRemoteFile(testFile, []byte(content1)); err != nil {
		return err
	}

	r.Log("Étape 2: Sync et hydratation")
	if err := r.RunSync(ctx); err != nil {
		return err
	}
	data, err := r.ReadLocalFile(testFile)
	if err != nil {
		return err
	}
	if string(data) != content1 {
		return r.Errorf("contenu initial incorrect")
	}

	r.Log("Étape 3: Modifier fichier distant")
	if err := r.CreateRemoteFile(testFile, []byte(content2)); err != nil {
		return err
	}

	r.Log("Étape 4: Sync")
	if err := r.RunSync(ctx); err != nil {
		return err
	}

	r.Log("Étape 5: Vérifier contenu local mis à jour")
	data, err = r.ReadLocalFile(testFile)
	if err != nil {
		return err
	}
	if string(data) != content2 {
		return r.Errorf("contenu non mis à jour: got %q, want %q", string(data), content2)
	}

	return nil
}

// T8: Remote Delete
// 1. Create file on remote, sync
// 2. Delete file on remote
// 3. Sync
// 4. Verify local file is deleted
func runT8RemoteDelete(ctx context.Context, r *Runner) error {
	const testFile = "t8_delete_test.txt"
	const content = "File to delete T8"

	r.Log("Étape 1: Créer fichier distant")
	if err := r.CreateRemoteFile(testFile, []byte(content)); err != nil {
		return err
	}

	r.Log("Étape 2: Sync pour créer placeholder")
	if err := r.RunSync(ctx); err != nil {
		return err
	}

	r.Log("Étape 3: Vérifier placeholder existe")
	if err := r.VerifyPlaceholderExists(testFile); err != nil {
		return err
	}

	r.Log("Étape 4: Supprimer fichier distant")
	if err := r.DeleteRemoteFile(testFile); err != nil {
		return err
	}

	r.Log("Étape 5: Sync")
	if err := r.RunSync(ctx); err != nil {
		return err
	}

	r.Log("Étape 6: Vérifier fichier local supprimé")
	return r.VerifyFileDeleted(testFile)
}
