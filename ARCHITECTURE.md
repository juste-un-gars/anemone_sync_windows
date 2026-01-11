# Architecture AnemoneSync
**Type**: OneDrive-like Smart Sync pour SMB
**Décision**: Option B - Client SMB Intégré (2026-01-11)

---

## Vision Globale

AnemoneSync fonctionne comme **OneDrive/Dropbox** mais pour des serveurs SMB :
- Le serveur SMB est la **source de vérité**
- Cache local intelligent avec **états de fichiers**
- Sync **à la demande** et **sélective**
- Mode **offline** avec queue de synchronisation

```
Serveur SMB (Source de vérité)
    ↓
AnemoneSync Client (Cache intelligent)
    ↓
Fichiers Locaux
    ├─ ✅ Synced (disponible offline)
    ├─ ☁️ Cloud-only (métadonnées seulement)
    ├─ 🔄 Syncing (en cours)
    └─ 📌 Pinned (toujours gardé en cache)
```

---

## Phases de Développement

### ✅ Phase 0 : Infrastructure (Session 001)
- Configuration système (Viper + YAML)
- Database SQLite + SQLCipher
- Logging avec Zap
- CI/CD GitHub Actions
- Documentation complète

### ✅ Phase 1 : Scanner de Fichiers (Session 002)
**Modules** : `internal/scanner/`
- Scan récursif avec exclusions (3 niveaux)
- Calcul hash SHA256 optimisé (chunked 4MB)
- Détection changements (algorithme 3-step)
- Worker pool (4 workers parallèles)
- 65+ tests, ~80% coverage

**Algorithme 3-Step** :
```
1. Comparer size + mtime (rapide)
2. Si changement → calculer hash SHA256
3. Comparer hash avec DB
→ 95%+ des fichiers skippés (unchanged)
```

### 🔥 Phase 2 : Client SMB + Authentification (Session 003+)
**Modules** : `internal/smb/`

#### 2.1 Connexion SMB
```go
type SMBClient struct {
    session     *smb2.Session
    server      string
    share       string
    credentials *Credentials
}

func (c *SMBClient) Connect() error
func (c *SMBClient) Disconnect() error
func (c *SMBClient) IsConnected() bool
```

#### 2.2 Opérations Fichiers
```go
func (c *SMBClient) Download(remote, local string) error
func (c *SMBClient) Upload(local, remote string) error
func (c *SMBClient) Delete(remote string) error
func (c *SMBClient) ListRemote(path string) ([]FileInfo, error)
func (c *SMBClient) GetRemoteMetadata(path string) (*FileMetadata, error)
```

#### 2.3 Gestion Credentials
- Stockage sécurisé dans Windows Credential Manager
- Chiffrement via `github.com/zalando/go-keyring`
- Support username/password + domaine
- Tests avec mock SMB server

**Durée estimée** : 6-8h

### 🔥 Phase 3 : Cache Intelligent + États Fichiers (Session 004+)
**Modules** : `internal/cache/`, `internal/vfs/`

#### 3.1 Cache Local avec LRU
```go
type FileCache struct {
    maxSize       int64         // Taille max (ex: 50GB)
    currentSize   int64         // Taille actuelle
    lruPolicy     *LRU          // Least Recently Used
    pinnedFiles   map[string]bool  // Fichiers épinglés
}

func (c *FileCache) Get(path string) (*File, error)
func (c *FileCache) Evict(size int64) error  // Libérer espace
func (c *FileCache) Pin(path string) error   // Épingler
func (c *FileCache) Unpin(path string) error
```

#### 3.2 États de Fichiers
```go
type FileState int
const (
    CloudOnly         FileState = iota  // ☁️ Serveur seulement
    AvailableOffline                    // ✅ Toujours en cache
    LocallyAvailable                    // 📁 En cache, peut être évict
    Syncing                             // 🔄 En cours de sync
    Error                               // ❌ Erreur
    Pinned                              // 📌 Épinglé
)
```

#### 3.3 Virtual File System
```go
type VirtualFileSystem struct {
    cache    *FileCache
    smb      *SMBClient
    metadata *MetadataDB
}

// Hydratation à la demande
func (vfs *VirtualFileSystem) HydrateFile(path string) error {
    if !vfs.cache.Has(path) {
        return vfs.smb.Download(path, vfs.cache.LocalPath(path))
    }
}

// Libérer espace
func (vfs *VirtualFileSystem) FreeUpSpace(path string) error {
    if vfs.cache.CanEvict(path) {
        return vfs.cache.Remove(path)
    }
}
```

**Durée estimée** : 8-10h

### 🔥 Phase 4 : Sync Engine Bidirectionnel (Session 005+)
**Modules** : `internal/sync/`

#### 4.1 Détection Changements
```go
type SyncEngine struct {
    scanner    *scanner.Scanner
    smb        *SMBClient
    cache      *FileCache
    offline    *OfflineQueue
}

func (s *SyncEngine) DetectLocalChanges() ([]Change, error)
func (s *SyncEngine) DetectRemoteChanges() ([]Change, error)
func (s *SyncEngine) DetectConflicts(local, remote []Change) []Conflict
```

#### 4.2 Résolution Conflits
```go
type ConflictResolution int
const (
    KeepRecent   ConflictResolution = iota  // Garder plus récent
    KeepLocal                                // Toujours local
    KeepRemote                               // Toujours remote
    KeepBoth                                 // Dupliquer
    AskUser                                  // Demander
)

func (s *SyncEngine) ResolveConflict(c Conflict, policy ConflictResolution) error
```

#### 4.3 Mode Offline
```go
type OfflineQueue struct {
    db    *database.DB
    queue []*QueuedOperation
}

type QueuedOperation struct {
    Type     OperationType  // Upload, Download, Delete
    Path     string
    Priority int
    Retries  int
    Error    string
}

// Ajouter à la queue si offline
func (oq *OfflineQueue) Enqueue(op *QueuedOperation) error

// Rejouer quand online
func (oq *OfflineQueue) ProcessQueue() error
```

#### 4.4 Sync Sélective
```
Documents/           ✅ Toujours sync
  ├─ report.docx     ✅ Synced
  └─ archive.zip     ☁️ Cloud-only

Photos/              ☁️ Cloud-only par défaut
  ├─ vacation/       ✅ Dossier sélectionné
  └─ backup/         ☁️ Non sync
```

**Durée estimée** : 10-12h

### 💎 Phase 5 : Interface Utilisateur (Session 006+)
**Modules** : `internal/ui/` (Fyne)

#### 5.1 Tree View avec États
```
📁 Documents/
  ├─ ✅ report.docx       (2.5 MB, Synced)
  ├─ ☁️ presentation.pptx (15 MB, Cloud-only)
  └─ 🔄 data.xlsx         (1.2 MB, Syncing 45%)

📁 Photos/ (☁️ Cloud-only)
  └─ 📁 vacation/ (✅ Available offline)
```

#### 5.2 Context Menu
```
Right-click on file/folder:
├─ Make available offline
├─ Free up space
├─ Always keep on this device
├─ View online
└─ Properties (sync status, size, dates)
```

#### 5.3 Settings
```
Sync Settings:
├─ General
│   ├─ Auto-start with Windows
│   ├─ Run in background
│   └─ Notifications
├─ Account
│   ├─ SMB Server: \\server\share
│   ├─ Username: ***
│   └─ Test Connection
├─ Sync
│   ├─ Sync folders (select which folders)
│   ├─ Files on-demand (enable/disable)
│   └─ Network: WiFi only / WiFi + Mobile
└─ Storage
    ├─ Cache location: C:\Users\...\AnemoneCache
    ├─ Max cache size: 50 GB
    └─ Free up space now
```

#### 5.4 Status Bar / Tray Icon
```
System Tray:
🔄 Syncing... (3 files remaining)
✅ Up to date
⚠️ Sync paused (no network)
❌ Error: Cannot connect to server
```

**Durée estimée** : 8-10h

### 💎 Phase 6 : Watchers & Background Sync (Session 007+)
**Modules** : `internal/watcher/`, `internal/network/`

#### 6.1 File System Watcher
```go
type FileWatcher struct {
    watcher   *fsnotify.Watcher
    debounce  time.Duration  // 30s
    onChange  func(path string)
}

// Surveiller changements locaux
func (fw *FileWatcher) Watch(path string) error
```

#### 6.2 Network Monitor
```go
type NetworkMonitor struct {
    isOnline    bool
    onOnline    func()
    onOffline   func()
}

// Détecter connexion/déconnexion
func (nm *NetworkMonitor) Start() error
```

#### 6.3 Background Sync
```go
type BackgroundSync struct {
    interval   time.Duration  // 5 minutes
    syncEngine *SyncEngine
}

// Sync périodique en arrière-plan
func (bs *BackgroundSync) Start() error
```

**Durée estimée** : 6-8h

---

## Optimisations Performance

### Scanner (Phase 1) ✅
- **Skip rate 95%+** : Size+mtime check avant hash
- **Batch DB updates** : 100 fichiers groupés (80% plus rapide)
- **Pattern cache** : Regex pré-compilés
- **Chunked hashing** : 4MB buffers (jamais tout en mémoire)
- **Résultat** : 1000+ petits fichiers/sec, hash 100MB < 2s

### Cache (Phase 3) 🔜
- **LRU eviction** : Libérer espace intelligemment
- **Lazy loading** : Télécharger seulement si nécessaire
- **Metadata caching** : Éviter scan SMB répétés
- **Compression** : Optionnel pour économiser espace

### Sync (Phase 4) 🔜
- **Delta sync** : Seulement blocs modifiés (rsync-like)
- **Parallel transfers** : 4 fichiers simultanés
- **Throttling** : Limiter bande passante si configuré
- **Smart retry** : Exponential backoff

---

## Sécurité

### Credentials
- ✅ Stockage Windows Credential Manager
- ✅ Jamais en clair dans config/DB
- ✅ Chiffrement SQLCipher pour DB
- 🔜 Support Kerberos (future)

### Données
- ✅ DB chiffrée (SQLCipher)
- 🔜 Cache local optionnellement chiffré
- 🔜 SMB3 encryption forcée (si disponible)
- 🔜 Zero-knowledge option (chiffrer avant upload)

### Réseau
- 🔜 Validation certificats SMB
- 🔜 Retry avec backoff (pas flood server)
- 🔜 Rate limiting

---

## Différences vs Alternatives

### vs OneDrive
- ✅ Fonctionne avec n'importe quel serveur SMB
- ✅ Pas de limite taille/nb fichiers
- ✅ Open source
- ❌ Pas de collaboration temps réel

### vs Syncthing
- ✅ Interface familière (OneDrive-like)
- ✅ États fichiers visuels
- ✅ Serveur SMB existant (pas besoin installer agent)
- ❌ Pas de P2P

### vs rclone
- ✅ Interface graphique intuitive
- ✅ Intégration système (icônes, tray)
- ✅ Mode offline intelligent
- ❌ Moins de backends supportés

---

## Stack Technique

### Backend
- **Go 1.21+** : Performance, cross-platform
- **go-smb2** : Client SMB natif
- **SQLite + SQLCipher** : DB chiffrée
- **Viper** : Configuration
- **Zap** : Logging haute performance
- **fsnotify** : File system watcher

### UI
- **Fyne v2** : GUI cross-platform native
- **Systray** : Icône système
- **Notifications** : Toast Windows

### Tests
- **Go testing** : Tests unitaires
- **testify** : Assertions
- **gomock** : Mocking (SMB, etc.)

---

## Estimations

### Temps Total MVP
- Phase 0 (Infrastructure) : ✅ 2h
- Phase 1 (Scanner) : ✅ 4h
- Phase 2 (SMB Client) : 🔜 6-8h
- Phase 3 (Cache/VFS) : 🔜 8-10h
- Phase 4 (Sync Engine) : 🔜 10-12h
- Phase 5 (UI Basic) : 🔜 8-10h
- Phase 6 (Watchers) : 🔜 6-8h

**Total MVP** : ~50-60h

### Lignes de Code Estimées
- Phase 1 : ✅ ~4100 lignes
- Phase 2-6 : 🔜 ~8000 lignes
- **Total** : ~12000-15000 lignes

---

## Prochaines Étapes Immédiates

### Session 003 (Prochain)
1. ✅ Finaliser tests worker pool (15 min)
2. 🔥 Commencer Phase 2 : Client SMB
   - Setup go-smb2
   - Connexion basique
   - Download/Upload simple
   - Tests avec mock

### Session 004
3. Authentification sécurisée (keystore)
4. Retry + error handling
5. Scan remote SMB

### Session 005
6. Phase 3 : Cache LRU
7. États fichiers
8. Metadata DB

---

**Document maintenu par** : Claude Sonnet 4.5
**Dernière mise à jour** : 2026-01-11
**Version** : 0.1.0-dev (Phase 1 complète)
