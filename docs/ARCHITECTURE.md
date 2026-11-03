# Architecture

```
 Phone (Safari / home screen app)                 Windows PC (desklink.exe)
 ┌─────────────────────────────┐          ┌──────────────────────────────────┐
 │ web/static  (HTML, CSS, JS) │ ─HTTP──▶ │ net/http (embedded files)        │
 │                             │ ─POST──▶ │ /api/pair  → internal/auth       │
 │ WebSocket client            │ ◀──WS──▶ │ /ws        → internal/server     │
 └─────────────────────────────┘          │   press "id" → config → actions  │
                                          │   live data  ← stats, media      │
 Dashboard window (Edge app mode)         │                                  │
 ┌─────────────────────────────┐          │ /admin     → localhost only      │
 │ web/admin.html              │ ─HTTP──▶ │ tray icon  → internal/tray       │
 └─────────────────────────────┘          └──────────────────────────────────┘
```

## Layout

| Path | Responsibility |
|---|---|
| `cmd/desklink` | Entry point: log file, single instance, HTTP server, tray icon, start with Windows. |
| `internal/config` | Parses and validates `config.json`. Exposes only the public part of the layout to phones. |
| `internal/actions` | The fixed set of action types and their Windows implementation (`ShellExecuteW`, `keybd_event`, `LockWorkStation`, `SetSuspendState`). |
| `internal/auth` | Rotating pairing codes (30 s), handoff codes, device tokens, `devices.json`. |
| `internal/server` | HTTP routes, WebSocket protocol, live streams, LAN-only guard, security headers, the dashboard API. |
| `internal/art` | Game pictures: custom covers, Steam cover art, icons extracted from executables. |
| `internal/media` | Now playing (Windows media sessions, via WinRT) and the system volume (Core Audio). |
| `internal/stats` | CPU, RAM and NVIDIA GPU readings (NVML). |
| `internal/procstat` | DeskLink's own memory and CPU use, for the dashboard. |
| `internal/tray`, `internal/desktop` | Notification-area icon and menu, dashboard window, dialogs, start with Windows. |
| `web/static` | The phone panel. `web/admin.html` is the dashboard. Both are embedded into the executable with `go:embed`. |

Windows APIs are called directly through `syscall`, with no cgo. The only dependency outside the Go standard library is [`github.com/coder/websocket`](https://github.com/coder/websocket).

## Connection flow

1. The phone loads `/` over HTTP. The page reads its token from `localStorage`.
2. **No token:** the page posts a pairing code to `/api/pair`, either from the QR code (the page was opened with `#pair=<code>`) or typed in. It stores the returned token. In Safari on iOS it also puts the returned single-use handoff code in its URL (`#h=<code>`), so a home screen app added from that page can fetch the same token.
3. The page opens `ws://<host>/ws` and sends `{"type":"auth","token":"…"}` as its first message (within 5 s).
4. The server replies with the layout: groups of buttons and the games with their art. Action targets never leave the PC.
5. A tap sends `{"type":"press","id":"…"}` and the server answers with a `result`. Dragging the volume slider sends `{"type":"volume","level":0-100}`.
6. The page turns live data on and off for the screen it is showing: `{"type":"media","on":true}` on the home screen, `{"type":"stats","on":true}` on the stats screen (and for the home screen's CPU/GPU rings). Nothing is sampled while no phone asks for it.
7. If the token is unknown, the server sends `{"type":"unauthorized"}`. The page deletes the token and shows the pairing screen.

Keeping the connection alive:

- The server pings every 30 s and drops connections that do not answer.
- The page reconnects with backoff (1 s, 2 s, 4 s … up to 10 s), and immediately when it becomes visible again or the network comes back.

## Why these choices

- **Go:** a single static executable that people can run without installing anything, with a small memory footprint and no bundled interpreter.
- **No frontend framework:** the reference device is an iPhone 6s on iOS 15. Plain ES2017 keeps the panel small and fast, and animations only use `transform` and `opacity`.
- **WebSocket:** the lowest latency per press, and the server can push now playing and stats without polling.
- **No service worker:** iOS only allows service workers on HTTPS origins, and a LAN address over HTTP cannot have one.
