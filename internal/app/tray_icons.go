// Package app provides tray icon generation with status overlays.
package app

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	"fyne.io/fyne/v2"
)

// TrayIconState represents the current state of the tray icon.
type TrayIconState int

const (
	TrayIconNormal  TrayIconState = iota // Default state (no overlay)
	TrayIconSyncing                      // Blue pulsing indicator
	TrayIconError                        // Red error indicator
	TrayIconWarning                      // Orange warning indicator
)

// trayIcons holds pre-generated icon resources for each state.
type trayIcons struct {
	normal  fyne.Resource
	syncing fyne.Resource
	error   fyne.Resource
	warning fyne.Resource
}

// generateTrayIcons creates all icon variants from the base icon.
func generateTrayIcons(baseIconData []byte) (*trayIcons, error) {
	// Decode base PNG
	baseImg, err := png.Decode(bytes.NewReader(baseIconData))
	if err != nil {
		return nil, err
	}

	icons := &trayIcons{}

	// Normal icon (no overlay)
	icons.normal = fyne.NewStaticResource("icon_normal.png", baseIconData)

	// Syncing icon (blue badge)
	syncingImg := addBadge(baseImg, color.RGBA{33, 150, 243, 255}) // Material Blue
	icons.syncing = pngToResource("icon_syncing.png", syncingImg)

	// Error icon (red badge)
	errorImg := addBadge(baseImg, color.RGBA{244, 67, 54, 255}) // Material Red
	icons.error = pngToResource("icon_error.png", errorImg)

	// Warning icon (orange badge)
	warningImg := addBadge(baseImg, color.RGBA{255, 152, 0, 255}) // Material Orange
	icons.warning = pngToResource("icon_warning.png", warningImg)

	return icons, nil
}

// addBadge adds a colored circular badge to the bottom-right corner of the image.
func addBadge(baseImg image.Image, badgeColor color.RGBA) image.Image {
	bounds := baseImg.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Create new RGBA image
	result := image.NewRGBA(bounds)
	draw.Draw(result, bounds, baseImg, bounds.Min, draw.Src)

	// Badge size: ~25% of icon size
	badgeRadius := width / 8
	if badgeRadius < 4 {
		badgeRadius = 4
	}

	// Badge position: bottom-right corner with small margin
	centerX := width - badgeRadius - 2
	centerY := height - badgeRadius - 2

	// Draw filled circle (badge)
	drawFilledCircle(result, centerX, centerY, badgeRadius, badgeColor)

	// Draw white border around badge for visibility
	drawCircleBorder(result, centerX, centerY, badgeRadius, color.RGBA{255, 255, 255, 255}, 2)

	return result
}

// drawFilledCircle draws a filled circle on the image.
func drawFilledCircle(img *image.RGBA, cx, cy, radius int, c color.RGBA) {
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy <= radius*radius {
				if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
					img.Set(x, y, c)
				}
			}
		}
	}
}

// drawCircleBorder draws a circle border on the image.
func drawCircleBorder(img *image.RGBA, cx, cy, radius int, c color.RGBA, thickness int) {
	outerRadius := radius + thickness
	innerRadius := radius

	for y := cy - outerRadius; y <= cy+outerRadius; y++ {
		for x := cx - outerRadius; x <= cx+outerRadius; x++ {
			dx := x - cx
			dy := y - cy
			distSq := dx*dx + dy*dy
			if distSq <= outerRadius*outerRadius && distSq >= innerRadius*innerRadius {
				if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
					img.Set(x, y, c)
				}
			}
		}
	}
}

// pngToResource encodes an image to PNG and wraps it as a Fyne resource.
func pngToResource(name string, img image.Image) fyne.Resource {
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return fyne.NewStaticResource(name, buf.Bytes())
}
