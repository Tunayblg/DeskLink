//go:build !windows

package tray

var quit = make(chan struct{})

// Run blocks until Quit; there is no tray icon on this platform yet.
func Run(Options) error {
	<-quit
	return nil
}

func Quit() {
	select {
	case <-quit:
	default:
		close(quit)
	}
}
