module github.com/juste-un-gars/anemone_sync_windows

go 1.21

require (
	github.com/hirochachacha/go-smb2 v1.1.0
	github.com/mutecomm/go-sqlcipher/v4 v4.4.2
	github.com/zalando/go-keyring v0.2.3
	github.com/fsnotify/fsnotify v1.7.0
	fyne.io/fyne/v2 v2.4.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/spf13/viper v1.18.2
	go.uber.org/zap v1.26.0
	golang.org/x/crypto v0.17.0
	golang.org/x/sys v0.15.0
)

// Note: Les versions exactes et les dépendances transitives
// seront automatiquement résolues lors du premier 'go mod download'
