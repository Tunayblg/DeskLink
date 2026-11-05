# Architecture

```
 Phone: Safari, a home screen app,           Windows PC (desklink.exe)
 or the Android app                          ┌────────────────────────────────────┐
┌───────────────────────────────┐            │ net/http (embedded files)          │
│ web/static  (HTML, CSS, JS)   │ ─HTTP───▶  │ /api/pair  → internal/auth         │
│                               │ ─POST───▶  │ /ws        → internal/server       │
│ WebSocket client              │ ◀──WS───▶  │   press "id" → config → actions    │
└───────────────────────────────┘            │   live data  ← stats, media        │
                                             │   brightness → internal/brightness │
 Dashboard window (Edge app mode)            │                                    │
┌───────────────────────────────┐            │ /admin     → localhost only        │
│ web/admin  (dashboard, guide) │ ─HTTP───▶  │ tray icon  → internal/tray         │
└───────────────────────────────┘            └────────────────────────────────────┘
```

## Layout

| Path | Responsibility |
|---|---|
| `cmd/desklink` | Entry point: log file, single instance, HTTP server, tray icon, start with Windows. |
| `internal/config` | Parses, validates and migrates `config.json`: buttons, games and the six home screen tiles. Exposes only the public part of the layout to phones. |
| `internal/actions` | The fixed set of action types and their Windows implementation (`ShellExecuteW`, `keybd_event`, `LockWorkStation`, `SetSuspendState`). |
| `internal/auth` | Rotating pairing codes (30 s), handoff codes, device tokens, `devices.json`. |
| `internal/server` | HTTP routes, WebSocket protocol, live streams, watch mode, LAN-only guard, security headers, the dashboard API. |
| `internal/art` | Game pictures: custom covers, Steam cover art, icons extracted from executables. |
| `internal/media` | Now playing (Windows media sessions, via WinRT), the system volume and the per-program mixer (Core Audio). |
| `internal/brightness` | Screen brightness: DDC/CI for monitors, Windows' own control for laptop panels, and a see-through dimming window for every other screen. |
| `internal/stats` | CPU, RAM and NVIDIA GPU readings (NVML). |
| `internal/procstat` | DeskLink's own memory and CPU use, for the dashboard. |
| `internal/pointer` | The trackpad: relative mouse moves, clicks and scrolling. |
| `internal/tray`, `internal/desktop` | Notification-area icon and menu, dashboard window, dialogs, start with Windows. |
| `web/static` | The phone panel. |
| `web/admin` | The dashboard and its two user guides (iPhone and Android). Both folders are embedded into the executable with `go:embed`. |
| `android` | The Android app: a full-screen WebView onto the panel, with a connection screen that scans the QR code or takes the address by hand. |

Windows APIs are called directly through `syscall`, with no cgo. Outside the Go standard library there are only two small dependencies: [`github.com/coder/websocket`](https://github.com/coder/websocket) for the WebSocket and [`rsc.io/qr`](https://pkg.go.dev/rsc.io/qr) for the pairing QR code.

## Connection flow

1. The phone loads `/` over HTTP. The page reads its token from `localStorage`.
2. **No token:** the page posts a pairing code to `/api/pair`, either from the QR code (the page was opened with `#pair=<code>`) or typed in. It stores the returned token. In Safari on iOS it also puts the returned single-use handoff code in its URL (`#h=<code>`), so a home screen app added from that page can fetch the same token.
3. The page opens `ws://<host>/ws` and sends `{"type":"auth","token":"…"}` as its first message (within 5 s).
4. The server replies with the layout: groups of buttons, the games with their art, and the home screen tiles. Action targets (paths, addresses) never leave the PC.
5. A tap sends `{"type":"press","id":"…"}` and the server answers with a `result`. The other messages a phone can send are just as narrow: a volume level (0–100) for the PC or a program in the mixer, a screen brightness (0–100), one of the three watch mode buttons (`playpause`, `back`, `forward`), and trackpad moves.
6. The page turns live data on and off for the screen it is showing: `{"type":"media","on":true}` on the home screen, `{"type":"stats","on":true}` in game mode, `{"type":"mixer","on":true}` in game and watch mode. Nothing is sampled while no phone asks for it.
7. If the token is unknown, the server sends `{"type":"unauthorized"}`. The page deletes the token and shows the pairing screen.

Keeping the connection alive:

- The page pings every 15 s, and the server drops a connection that stays silent for 45 s (a phone that went to sleep or left the Wi-Fi). WebSocket protocol pings are not used: Safari on iOS 15 does not answer them.
- The page reconnects with backoff (1 s, 2 s, 4 s … up to 10 s), and immediately when it becomes visible again or the network comes back.

## Why these choices

- **Go:** a single static executable that people can run without installing anything, with a small memory footprint and no bundled interpreter.
- **No frontend framework:** the reference device is an iPhone 6s on iOS 15. Plain ES2017 keeps the panel small and fast, and animations only use `transform` and `opacity`.
- **WebSocket:** the lowest latency per press, and the server can push now playing and stats without polling.
- **No service worker:** iOS only allows service workers on HTTPS origins, and a LAN address over HTTP cannot have one. For the same reason Android browsers will not install the panel as a full-screen app, which is why there is a small Android app.
- **Keys for watch mode:** video players have no common API for seeking, but nearly all of them jump with the arrow keys, so that is what the PC is sent.
