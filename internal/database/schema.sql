-- Schéma de base de données SQLite pour AnemoneSync
-- Version: 1.0
-- Cette base de données sera chiffrée avec SQLCipher

-- Table des jobs de synchronisation
CREATE TABLE IF NOT EXISTS sync_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    local_path TEXT NOT NULL,
    remote_path TEXT NOT NULL,
    server_credential_id TEXT NOT NULL, -- Référence au keystore système
    sync_mode TEXT NOT NULL CHECK(sync_mode IN ('mirror', 'upload', 'download', 'mirror_priority')),
    trigger_mode TEXT NOT NULL CHECK(trigger_mode IN ('realtime', 'interval', 'scheduled', 'manual')),
    trigger_params TEXT, -- JSON: délai, intervalle, horaires, etc.
    conflict_resolution TEXT CHECK(conflict_resolution IN ('recent', 'local', 'remote', 'both', 'ask')),
    network_conditions TEXT, -- JSON: wifi, data, specific_networks
    enabled INTEGER NOT NULL DEFAULT 1,
    last_run INTEGER, -- Unix timestamp
    next_run INTEGER, -- Unix timestamp
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(local_path, remote_path)
);

-- Index pour recherches fréquentes
CREATE INDEX IF NOT EXISTS idx_sync_jobs_enabled ON sync_jobs(enabled);
CREATE INDEX IF NOT EXISTS idx_sync_jobs_next_run ON sync_jobs(next_run) WHERE enabled = 1;

-- Table d'état des fichiers
CREATE TABLE IF NOT EXISTS files_state (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL,
    local_path TEXT NOT NULL,
    remote_path TEXT NOT NULL,
    size INTEGER NOT NULL,
    mtime INTEGER NOT NULL, -- Unix timestamp de modification
    hash TEXT, -- SHA256 du contenu
    last_sync INTEGER, -- Unix timestamp
    sync_status TEXT NOT NULL CHECK(sync_status IN ('idle', 'syncing', 'error', 'queued')),
    error_message TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (job_id) REFERENCES sync_jobs(id) ON DELETE CASCADE,
    UNIQUE(job_id, local_path)
);

-- Index pour recherches et jointures
CREATE INDEX IF NOT EXISTS idx_files_state_job_id ON files_state(job_id);
CREATE INDEX IF NOT EXISTS idx_files_state_status ON files_state(sync_status);
CREATE INDEX IF NOT EXISTS idx_files_state_hash ON files_state(hash);

-- Table des exclusions
CREATE TABLE IF NOT EXISTS exclusions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK(type IN ('global', 'job', 'individual')),
    pattern_or_path TEXT NOT NULL,
    reason TEXT,
    date_added INTEGER NOT NULL,
    job_id INTEGER, -- NULL si global
    created_at INTEGER NOT NULL,
    FOREIGN KEY (job_id) REFERENCES sync_jobs(id) ON DELETE CASCADE
);

-- Index pour recherches d'exclusions
CREATE INDEX IF NOT EXISTS idx_exclusions_type ON exclusions(type);
CREATE INDEX IF NOT EXISTS idx_exclusions_job_id ON exclusions(job_id);

-- Table d'historique des synchronisations
CREATE TABLE IF NOT EXISTS sync_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL,
    timestamp INTEGER NOT NULL,
    files_synced INTEGER NOT NULL DEFAULT 0,
    files_failed INTEGER NOT NULL DEFAULT 0,
    bytes_transferred INTEGER NOT NULL DEFAULT 0,
    duration INTEGER NOT NULL, -- En secondes
    status TEXT NOT NULL CHECK(status IN ('success', 'partial', 'failed')),
    error_summary TEXT,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (job_id) REFERENCES sync_jobs(id) ON DELETE CASCADE
);

-- Index pour historique et statistiques
CREATE INDEX IF NOT EXISTS idx_sync_history_job_id ON sync_history(job_id);
CREATE INDEX IF NOT EXISTS idx_sync_history_timestamp ON sync_history(timestamp);
CREATE INDEX IF NOT EXISTS idx_sync_history_status ON sync_history(status);

-- Table des serveurs SMB (pour référence, credentials dans keystore)
CREATE TABLE IF NOT EXISTS smb_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER DEFAULT 445,
    share TEXT NOT NULL,
    domain TEXT,
    credential_id TEXT NOT NULL UNIQUE, -- ID dans le keystore système
    smb_version TEXT CHECK(smb_version IN ('2.0', '2.1', '3.0', '3.1.1')),
    last_connection_test INTEGER, -- Unix timestamp
    last_connection_status TEXT CHECK(last_connection_status IN ('success', 'failed')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(host, share)
);

-- Index pour serveurs
CREATE INDEX IF NOT EXISTS idx_smb_servers_credential_id ON smb_servers(credential_id);

-- Table de configuration (key-value store)
CREATE TABLE IF NOT EXISTS app_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    value_type TEXT NOT NULL CHECK(value_type IN ('string', 'int', 'bool', 'json')),
    description TEXT,
    updated_at INTEGER NOT NULL
);

-- Table de la file d'attente hors-ligne
CREATE TABLE IF NOT EXISTS offline_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    operation TEXT NOT NULL CHECK(operation IN ('upload', 'download', 'delete')),
    priority INTEGER NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (job_id) REFERENCES sync_jobs(id) ON DELETE CASCADE
);

-- Index pour la file d'attente
CREATE INDEX IF NOT EXISTS idx_offline_queue_job_id ON offline_queue(job_id);
CREATE INDEX IF NOT EXISTS idx_offline_queue_priority ON offline_queue(priority DESC);

-- Table de métadonnées de la base de données
CREATE TABLE IF NOT EXISTS db_metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Insertion des métadonnées initiales
INSERT OR IGNORE INTO db_metadata (key, value) VALUES ('schema_version', '1');
INSERT OR IGNORE INTO db_metadata (key, value) VALUES ('created_at', strftime('%s', 'now'));
INSERT OR IGNORE INTO db_metadata (key, value) VALUES ('app_version', '0.1.0-dev');

-- Vue pour statistiques rapides par job
CREATE VIEW IF NOT EXISTS job_statistics AS
SELECT
    sj.id,
    sj.name,
    sj.enabled,
    COUNT(DISTINCT fs.id) as total_files,
    SUM(fs.size) as total_size,
    MAX(fs.last_sync) as last_sync_time,
    (SELECT COUNT(*) FROM files_state WHERE job_id = sj.id AND sync_status = 'error') as files_with_errors,
    (SELECT COUNT(*) FROM offline_queue WHERE job_id = sj.id) as queued_operations
FROM sync_jobs sj
LEFT JOIN files_state fs ON fs.job_id = sj.id
GROUP BY sj.id;

-- Vue pour historique récent (30 derniers jours)
CREATE VIEW IF NOT EXISTS recent_sync_history AS
SELECT
    sh.*,
    sj.name as job_name
FROM sync_history sh
JOIN sync_jobs sj ON sj.id = sh.job_id
WHERE sh.timestamp >= strftime('%s', 'now', '-30 days')
ORDER BY sh.timestamp DESC;

-- Triggers pour mettre à jour automatiquement les timestamps
CREATE TRIGGER IF NOT EXISTS update_sync_jobs_timestamp
AFTER UPDATE ON sync_jobs
BEGIN
    UPDATE sync_jobs SET updated_at = strftime('%s', 'now') WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_files_state_timestamp
AFTER UPDATE ON files_state
BEGIN
    UPDATE files_state SET updated_at = strftime('%s', 'now') WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_smb_servers_timestamp
AFTER UPDATE ON smb_servers
BEGIN
    UPDATE smb_servers SET updated_at = strftime('%s', 'now') WHERE id = NEW.id;
END;

-- Trigger pour nettoyer les anciennes entrées de l'historique (conservation: 90 jours)
CREATE TRIGGER IF NOT EXISTS cleanup_old_sync_history
AFTER INSERT ON sync_history
BEGIN
    DELETE FROM sync_history
    WHERE timestamp < strftime('%s', 'now', '-90 days');
END;
