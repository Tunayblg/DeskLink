// Command desklink runs the DeskLink server with a system tray icon.
//
// Release builds are linked with -H windowsgui, so there is no console window:
// output goes to desklink.log in the data folder, and fatal errors are shown
// in a dialog.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Tunayblg/DeskLink/internal/art"
	"github.com/Tunayblg/DeskLink/internal/auth"
	"github.com/Tunayblg/DeskLink/internal/config"
	"github.com/Tunayblg/DeskLink/internal/desktop"
	"github.com/Tunayblg/DeskLink/internal/server"
	"github.com/Tunayblg/DeskLink/internal/tray"
	"github.com/Tunayblg/DeskLink/web"
)

var version = "1.0.0"

const maxLog = 1 << 20

func main() {
	dataFlag := flag.String("data", "", "folder for config.json and devices.json (default: next to the executable)")
	background := flag.Bool("background", false, "start without opening the dashboard (used when starting with Windows)")
	flag.Parse()

	exe, err := os.Executable()
	if err != nil {
		fatal(err.Error())
	}
	dataDir := *dataFlag
	if dataDir == "" {
		dataDir = filepath.Dir(exe)
	}
	setupLog(filepath.Join(dataDir, "desklink.log"))
	log.Printf("DeskLink %s starting (data: %s)", version, dataDir)

	cfg, err := config.Load(filepath.Join(dataDir, "config.json"))
	if err != nil {
		fatal("config.json: " + err.Error())
	}
	dashboard := fmt.Sprintf("http://localhost:%d/admin", cfg.Port)

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		// Most likely DeskLink is already running: show its dashboard instead.
		if isDeskLink(cfg.Port) {
			log.Printf("already running; opening the dashboard")
			if !*background {
				desktop.OpenApp(dashboard)
			}
			return
		}
		fatal(fmt.Sprintf("Port %d is used by another program. Change \"port\" in config.json.\n\n%v", cfg.Port, err))
	}

	store, err := auth.Open(filepath.Join(dataDir, "devices.json"), func(pin string) {
		log.Printf("Pairing PIN: %s", pin)
	})
	if err != nil {
		fatal("devices.json: " + err.Error())
	}

	app := server.New(cfg, store, web.Files, art.Load(cfg.Games))
	app.Version = version
	app.AdminPage = web.AdminPage
	for _, ip := range lanIPs() {
		app.Addresses = append(app.Addresses, fmt.Sprintf("http://%s:%d", ip, cfg.Port))
	}
	srv := &http.Server{
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal(err.Error())
		}
	}()
	log.Printf("phone addresses: %s", strings.Join(app.Addresses, ", "))
	log.Printf("dashboard: %s", dashboard)

	// Ctrl+C when run from a terminal during development.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() { <-ctx.Done(); tray.Quit() }()

	if !*background {
		go desktop.OpenApp(dashboard)
	}

	// Start-with-Windows runs the same executable and data folder, in the background.
	autostartCmd := fmt.Sprintf("%q --background", exe)
	if *dataFlag != "" {
		autostartCmd += fmt.Sprintf(" -data %q", dataDir)
	}
	text := desktop.Text()
	err = tray.Run(tray.Options{
		Tooltip: "DeskLink",
		Icon:    web.TrayIcon,
		OnOpen:  func() { desktop.OpenApp(dashboard) },
		Items: []tray.Item{
			{Label: text.Open, OnClick: func() { desktop.OpenApp(dashboard) }},
			{Label: text.Autostart, Checked: desktop.Autostart, OnClick: func() {
				on := !desktop.Autostart()
				if err := desktop.SetAutostart(on, autostartCmd); err != nil {
					log.Printf("autostart: %v", err)
					desktop.Alert(err.Error())
					return
				}
				log.Printf("start with Windows: %v", on)
			}},
			{},
			{Label: text.Quit, OnClick: tray.Quit},
		},
	})
	if err != nil {
		log.Printf("tray: %v (running without an icon; stop with Ctrl+C)", err)
		<-ctx.Done()
	}

	log.Printf("shutting down")
	shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	srv.Shutdown(shutdown)
}

// isDeskLink reports whether a DeskLink server answers on port.
func isDeskLink(port int) bool {
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/manifest.json", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return strings.Contains(string(b), `"DeskLink"`)
}

// setupLog writes the log to path (and to the console, if there is one). The
// file is started over once it passes 1 MB.
func setupLog(path string) {
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if st, err := os.Stat(path); err == nil && st.Size() > maxLog {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return
	}
	// The file comes first: without a console, writes to stderr fail, and
	// MultiWriter stops at the first error.
	log.SetOutput(io.MultiWriter(f, os.Stderr))
}

func fatal(msg string) {
	log.Print(msg)
	desktop.Alert("DeskLink could not start.\n\n" + msg)
	os.Exit(1)
}

// lanIPs lists this PC's private IPv4 addresses, which is what the phone needs.
func lanIPs() []string {
	var out []string
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && n.IP.IsPrivate() {
				out = append(out, n.IP.String())
			}
		}
	}
	return out
}
