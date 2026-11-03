// Package desktop holds the small pieces of Windows integration the tray app
// needs: opening the dashboard window, error dialogs, and start-with-Windows.
package desktop

// Strings for the tray menu, in the Windows display language when we have it.
type Strings struct {
	Open, Autostart, Quit string
}

var turkish = Strings{Open: "Paneli aç", Autostart: "Windows ile başlat", Quit: "Çıkış"}
var english = Strings{Open: "Open dashboard", Autostart: "Start with Windows", Quit: "Quit"}
