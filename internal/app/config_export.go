package app

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/database"
	"go.uber.org/zap"
)

const exportVersion = 1

// ConfigExport represents the exported configuration structure.
type ConfigExport struct {
	Version    int               `json:"version"`
	App        string            `json:"app"`
	ExportedAt string            `json:"exported_at"`
	SMBServers []exportServer    `json:"smb_servers"`
	SyncJobs   []exportJob       `json:"sync_jobs"`
	Exclusions []exportExclusion `json:"exclusions"`
	AppConfig  map[string]string `json:"app_config"`
}

type exportServer struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Domain     string `json:"domain,omitempty"`
	SMBVersion string `json:"smb_version,omitempty"`
}

type exportJob struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	LocalPath          string `json:"local_path"`
	RemotePath         string `json:"remote_path"`
	ServerCredentialID string `json:"server_credential_id"`
	SyncMode           string `json:"sync_mode"`
	TriggerMode        string `json:"trigger_mode"`
	TriggerParams      string `json:"trigger_params,omitempty"`
	ConflictResolution string `json:"conflict_resolution,omitempty"`
	NetworkConditions  string `json:"network_conditions,omitempty"`
	Enabled            bool   `json:"enabled"`
}

type exportExclusion struct {
	ID            int64  `json:"id"`
	Type          string `json:"type"`
	PatternOrPath string `json:"pattern_or_path"`
	Reason        string `json:"reason,omitempty"`
	JobID         *int64 `json:"job_id,omitempty"`
}

// ExportConfig exports the application configuration to a JSON file.
func (a *App) ExportConfig(path string) error {
	if a.db == nil {
		return fmt.Errorf("database not available")
	}

	servers, err := a.db.GetAllSMBServers()
	if err != nil {
		return fmt.Errorf("get servers: %w", err)
	}

	jobs, err := a.db.GetAllSyncJobs()
	if err != nil {
		return fmt.Errorf("get jobs: %w", err)
	}

	exclusions, err := a.db.GetAllExclusions()
	if err != nil {
		return fmt.Errorf("get exclusions: %w", err)
	}

	appConfig, err := a.db.GetAllAppConfig()
	if err != nil {
		return fmt.Errorf("get app config: %w", err)
	}

	export := ConfigExport{
		Version:    exportVersion,
		App:        AppName,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		AppConfig:  appConfig,
	}

	for _, s := range servers {
		export.SMBServers = append(export.SMBServers, exportServer{
			ID:         s.ID,
			Name:       s.Name,
			Host:       s.Host,
			Port:       s.Port,
			Username:   s.Username,
			Domain:     s.Domain,
			SMBVersion: s.SMBVersion,
		})
	}

	for _, j := range jobs {
		export.SyncJobs = append(export.SyncJobs, exportJob{
			ID:                 j.ID,
			Name:               j.Name,
			LocalPath:          j.LocalPath,
			RemotePath:         j.RemotePath,
			ServerCredentialID: j.ServerCredentialID,
			SyncMode:           j.SyncMode,
			TriggerMode:        j.TriggerMode,
			TriggerParams:      j.TriggerParams,
			ConflictResolution: j.ConflictResolution,
			NetworkConditions:  j.NetworkConditions,
			Enabled:            j.Enabled,
		})
	}

	for _, e := range exclusions {
		export.Exclusions = append(export.Exclusions, exportExclusion{
			ID:            e.ID,
			Type:          e.Type,
			PatternOrPath: e.PatternOrPath,
			Reason:        e.Reason,
			JobID:         e.JobID,
		})
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	a.logger.Info("Configuration exported",
		zap.String("path", path),
		zap.Int("servers", len(export.SMBServers)),
		zap.Int("jobs", len(export.SyncJobs)),
		zap.Int("exclusions", len(export.Exclusions)),
	)

	return nil
}

// ImportResult holds the results of a config import.
type ImportResult struct {
	ServersImported    int
	ServersSkipped     int
	JobsImported       int
	JobsSkipped        int
	ExclusionsImported int
	ExclusionsSkipped  int
	ConfigKeysImported int
}

// ImportConfig imports application configuration from a JSON file.
func (a *App) ImportConfig(path string) (*ImportResult, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var export ConfigExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if export.Version < 1 || export.Version > exportVersion {
		return nil, fmt.Errorf("unsupported config version: %d (supported: 1-%d)", export.Version, exportVersion)
	}

	if export.App != AppName {
		return nil, fmt.Errorf("invalid config file: expected app %q, got %q", AppName, export.App)
	}

	result := &ImportResult{}

	// Import servers - build old ID -> new ID mapping
	serverIDMap := make(map[int64]int64)
	existingServers, err := a.db.GetAllSMBServers()
	if err != nil {
		return nil, fmt.Errorf("get existing servers: %w", err)
	}

	existingServerHosts := make(map[string]*database.SMBServer)
	for _, s := range existingServers {
		existingServerHosts[s.Host] = s
	}

	for _, es := range export.SMBServers {
		if existing, ok := existingServerHosts[es.Host]; ok {
			serverIDMap[es.ID] = existing.ID
			result.ServersSkipped++
			continue
		}

		dbServer := &database.SMBServer{
			Name:       es.Name,
			Host:       es.Host,
			Port:       es.Port,
			Username:   es.Username,
			Domain:     es.Domain,
			SMBVersion: es.SMBVersion,
		}
		if err := a.db.CreateSMBServer(dbServer); err != nil {
			a.logger.Warn("Failed to import server", zap.String("host", es.Host), zap.Error(err))
			continue
		}
		serverIDMap[es.ID] = dbServer.ID
		result.ServersImported++
	}

	// Import jobs - build old ID -> new ID mapping
	jobIDMap := make(map[int64]int64)
	existingJobs, err := a.db.GetAllSyncJobs()
	if err != nil {
		return nil, fmt.Errorf("get existing jobs: %w", err)
	}

	type jobKey struct {
		localPath, remotePath string
	}
	existingJobPaths := make(map[jobKey]*database.SyncJob)
	for _, j := range existingJobs {
		existingJobPaths[jobKey{j.LocalPath, j.RemotePath}] = j
	}

	for _, ej := range export.SyncJobs {
		key := jobKey{ej.LocalPath, ej.RemotePath}
		if existing, ok := existingJobPaths[key]; ok {
			jobIDMap[ej.ID] = existing.ID
			result.JobsSkipped++
			continue
		}

		dbJob := &database.SyncJob{
			Name:               ej.Name,
			LocalPath:          ej.LocalPath,
			RemotePath:         ej.RemotePath,
			ServerCredentialID: ej.ServerCredentialID,
			SyncMode:           ej.SyncMode,
			TriggerMode:        ej.TriggerMode,
			TriggerParams:      ej.TriggerParams,
			ConflictResolution: ej.ConflictResolution,
			NetworkConditions:  ej.NetworkConditions,
			Enabled:            ej.Enabled,
		}
		if err := a.db.CreateSyncJob(dbJob); err != nil {
			a.logger.Warn("Failed to import job", zap.String("name", ej.Name), zap.Error(err))
			continue
		}
		jobIDMap[ej.ID] = dbJob.ID
		result.JobsImported++
	}

	// Import exclusions - remap job IDs
	existingExclusions, err := a.db.GetAllExclusions()
	if err != nil {
		return nil, fmt.Errorf("get existing exclusions: %w", err)
	}

	type exclKey struct {
		pattern string
		jobID   int64 // 0 for global
	}
	existingExclPatterns := make(map[exclKey]bool)
	for _, e := range existingExclusions {
		var jid int64
		if e.JobID != nil {
			jid = *e.JobID
		}
		existingExclPatterns[exclKey{e.PatternOrPath, jid}] = true
	}

	for _, ee := range export.Exclusions {
		var newJobID *int64
		var jid int64
		if ee.JobID != nil {
			if mapped, ok := jobIDMap[*ee.JobID]; ok {
				newJobID = &mapped
				jid = mapped
			} else {
				result.ExclusionsSkipped++
				continue
			}
		}

		if existingExclPatterns[exclKey{ee.PatternOrPath, jid}] {
			result.ExclusionsSkipped++
			continue
		}

		excl := &database.Exclusion{
			Type:          ee.Type,
			PatternOrPath: ee.PatternOrPath,
			Reason:        ee.Reason,
			JobID:         newJobID,
		}
		if err := a.db.CreateExclusion(excl); err != nil {
			a.logger.Warn("Failed to import exclusion", zap.String("pattern", ee.PatternOrPath), zap.Error(err))
			continue
		}
		result.ExclusionsImported++
	}

	// Import app config
	for key, value := range export.AppConfig {
		if err := a.db.SetAppConfig(key, value, "string"); err != nil {
			a.logger.Warn("Failed to import config key", zap.String("key", key), zap.Error(err))
			continue
		}
		result.ConfigKeysImported++
	}

	// Reload app state from database
	a.loadSettingsFromDB()
	a.loadSMBConnectionsFromDB()
	a.loadJobsFromDB()

	a.logger.Info("Configuration imported",
		zap.String("path", path),
		zap.Int("servers_imported", result.ServersImported),
		zap.Int("servers_skipped", result.ServersSkipped),
		zap.Int("jobs_imported", result.JobsImported),
		zap.Int("jobs_skipped", result.JobsSkipped),
	)

	return result, nil
}
