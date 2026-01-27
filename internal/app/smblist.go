package app

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// SMBList displays a list of configured SMB connections.
type SMBList struct {
	app       *App
	list      *widget.List
	container *fyne.Container
	selected  int
}

// NewSMBList creates a new SMB connections list.
func NewSMBList(app *App) *SMBList {
	sl := &SMBList{
		app:      app,
		selected: -1,
	}

	sl.list = widget.NewList(
		func() int {
			return len(app.GetSMBConnections())
		},
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabel("Connection Name"),
				widget.NewLabel("server/share - user"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			connections := app.GetSMBConnections()
			if id >= len(connections) {
				return
			}
			conn := connections[id]

			vbox := obj.(*fyne.Container)
			nameLabel := vbox.Objects[0].(*widget.Label)
			detailLabel := vbox.Objects[1].(*widget.Label)

			nameLabel.SetText(conn.DisplayName())
			detailLabel.SetText(conn.Host + " - " + conn.Username)
		},
	)

	sl.list.OnSelected = func(id widget.ListItemID) {
		sl.selected = int(id)
	}

	sl.list.OnUnselected = func(id widget.ListItemID) {
		sl.selected = -1
	}

	sl.container = container.NewStack(sl.list)

	return sl
}

// Container returns the container widget.
func (sl *SMBList) Container() *fyne.Container {
	return sl.container
}

// GetSelected returns the currently selected SMB connection, or nil if none.
func (sl *SMBList) GetSelected() *SMBConnection {
	if sl.selected < 0 {
		return nil
	}

	connections := sl.app.GetSMBConnections()
	if sl.selected >= len(connections) {
		return nil
	}

	return connections[sl.selected]
}

// Refresh reloads the list data.
func (sl *SMBList) Refresh() {
	sl.list.Refresh()
}
