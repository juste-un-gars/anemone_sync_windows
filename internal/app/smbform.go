package app

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// SMBForm is a form for creating/editing SMB connections.
type SMBForm struct {
	app        *App
	connection *SMBConnection // nil for new connection
	onSave     func(*SMBConnection)
	dialog     dialog.Dialog // Reference to close on success

	// Form fields
	nameEntry     *widget.Entry
	hostEntry     *widget.Entry
	portEntry     *widget.Entry
	usernameEntry *widget.Entry
	passwordEntry *widget.Entry
	domainEntry   *widget.Entry
}

// NewSMBForm creates a new SMB connection form.
func NewSMBForm(app *App, conn *SMBConnection, onSave func(*SMBConnection)) *SMBForm {
	f := &SMBForm{
		app:        app,
		connection: conn,
		onSave:     onSave,
	}

	// Initialize form fields
	f.nameEntry = widget.NewEntry()
	f.nameEntry.SetPlaceHolder("My NAS Server")

	f.hostEntry = widget.NewEntry()
	f.hostEntry.SetPlaceHolder("192.168.1.100 or nas.local")

	f.portEntry = widget.NewEntry()
	f.portEntry.SetPlaceHolder("445")
	f.portEntry.SetText("445")

	f.usernameEntry = widget.NewEntry()
	f.usernameEntry.SetPlaceHolder("admin")

	f.passwordEntry = widget.NewPasswordEntry()
	f.passwordEntry.SetPlaceHolder("Password")

	f.domainEntry = widget.NewEntry()
	f.domainEntry.SetPlaceHolder("WORKGROUP (optional)")

	// Pre-fill if editing
	if conn != nil {
		f.nameEntry.SetText(conn.Name)
		f.hostEntry.SetText(conn.Host)
		if conn.Port > 0 {
			f.portEntry.SetText(strconv.Itoa(conn.Port))
		}
		f.usernameEntry.SetText(conn.Username)
		f.domainEntry.SetText(conn.Domain)
		// Password is not pre-filled for security
	}

	return f
}

// Show displays the form in a dialog.
func (f *SMBForm) Show(parent fyne.Window) {
	title := "Add SMB Server"
	if f.connection != nil {
		title = "Edit SMB Server"
	}

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Name", Widget: f.nameEntry, HintText: "Display name for this server"},
			{Text: "Host", Widget: f.hostEntry, HintText: "Server IP or hostname"},
			{Text: "Port", Widget: f.portEntry, HintText: "SMB port (default 445)"},
			{Text: "Username", Widget: f.usernameEntry, HintText: "Authentication username"},
			{Text: "Password", Widget: f.passwordEntry, HintText: "Password (stored securely)"},
			{Text: "Domain", Widget: f.domainEntry, HintText: "Domain or workgroup (optional)"},
		},
		OnSubmit: func() {
			f.save(parent)
		},
	}

	// Add test connection button
	testBtn := widget.NewButton("Test Connection", func() {
		f.testConnection(parent)
	})

	// Info message about Anemone Server
	infoLabel := widget.NewLabel("Tip: For faster sync, use Anemone Server on your NAS.\nWithout it, file scanning will be slower on large shares.")
	infoLabel.Wrapping = fyne.TextWrapWord
	infoLabel.TextStyle = fyne.TextStyle{Italic: true}

	content := container.NewVBox(
		form,
		widget.NewSeparator(),
		container.NewHBox(testBtn),
		widget.NewSeparator(),
		infoLabel,
	)

	f.dialog = dialog.NewCustom(title, "Cancel", content, parent)
	f.dialog.Resize(fyne.NewSize(400, 400))
	f.dialog.Show()
}

// save validates and saves the SMB connection.
func (f *SMBForm) save(parent fyne.Window) {
	// Validate required fields
	if f.hostEntry.Text == "" {
		dialog.ShowError(errFieldRequired("Host"), parent)
		return
	}
	if f.usernameEntry.Text == "" {
		dialog.ShowError(errFieldRequired("Username"), parent)
		return
	}
	if f.passwordEntry.Text == "" && f.connection == nil {
		dialog.ShowError(errFieldRequired("Password"), parent)
		return
	}

	// Parse port
	port := 445
	if f.portEntry.Text != "" {
		p, err := strconv.Atoi(f.portEntry.Text)
		if err != nil || p < 1 || p > 65535 {
			dialog.ShowError(errInvalidPort, parent)
			return
		}
		port = p
	}

	// Create or update connection
	conn := f.connection
	if conn == nil {
		conn = &SMBConnection{}
	}

	conn.Name = f.nameEntry.Text
	if conn.Name == "" {
		conn.Name = f.hostEntry.Text
	}
	conn.Host = f.hostEntry.Text
	conn.Port = port
	conn.Username = f.usernameEntry.Text
	conn.Domain = f.domainEntry.Text

	// Save credentials to keyring (password only in keyring)
	password := f.passwordEntry.Text
	if password != "" {
		if err := f.app.SaveSMBCredential(conn.Host, conn.Username, password, conn.Domain, port); err != nil {
			dialog.ShowError(err, parent)
			return
		}
	}

	// Save to database
	var err error
	if f.connection == nil {
		err = f.app.AddSMBConnection(conn)
	} else {
		err = f.app.UpdateSMBConnection(conn)
	}

	if err != nil {
		dialog.ShowError(err, parent)
		return
	}

	if f.onSave != nil {
		f.onSave(conn)
	}

	// Close the form dialog
	if f.dialog != nil {
		f.dialog.Hide()
	}

	dialog.ShowInformation("Success", "SMB server saved successfully", parent)
}

// testConnection tests the SMB connection.
func (f *SMBForm) testConnection(parent fyne.Window) {
	if f.hostEntry.Text == "" || f.usernameEntry.Text == "" {
		dialog.ShowError(errFieldRequired("Host and Username"), parent)
		return
	}

	password := f.passwordEntry.Text
	if password == "" && f.connection != nil {
		// Try to load password from keyring for existing connection
		creds, err := f.app.LoadSMBCredential(f.connection.Host)
		if err == nil && creds != nil {
			password = creds.Password
		}
	}

	if password == "" {
		dialog.ShowError(errFieldRequired("Password"), parent)
		return
	}

	port := 445
	if f.portEntry.Text != "" {
		p, _ := strconv.Atoi(f.portEntry.Text)
		if p > 0 {
			port = p
		}
	}

	// Show progress
	progress := dialog.NewProgressInfinite("Testing Connection", "Connecting to SMB server...", parent)
	progress.Show()

	go func() {
		err := f.app.TestSMBConnection(f.hostEntry.Text, f.usernameEntry.Text, password, f.domainEntry.Text, port)

		fyne.Do(func() {
			progress.Hide()

			if err != nil {
				dialog.ShowError(err, parent)
			} else {
				dialog.ShowInformation("Success", "Connection successful!", parent)
			}
		})
	}()
}

// Error helpers
func errFieldRequired(field string) error {
	return &formError{msg: field + " is required"}
}

var errInvalidPort = &formError{msg: "Port must be between 1 and 65535"}

type formError struct {
	msg string
}

func (e *formError) Error() string {
	return e.msg
}
