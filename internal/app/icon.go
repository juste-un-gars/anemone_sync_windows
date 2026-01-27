package app

import (
	_ "embed"
)

// iconData contains the application icon embedded from assets/anemone.png
//
//go:embed assets/anemone.png
var iconData []byte
