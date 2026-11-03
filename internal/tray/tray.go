// Package tray shows DeskLink's notification-area (system tray) icon.
package tray

// Item is a menu entry. Checked, if set, is asked each time the menu opens.
type Item struct {
	Label   string
	Checked func() bool
	OnClick func()
}

// Options configures the icon. OnOpen runs on a left click.
type Options struct {
	Tooltip string
	Icon    []byte // PNG, 32×32
	OnOpen  func()
	Items   []Item // a nil Item (zero Label) is a separator
}
