# Configuration

`config.json` is created next to `desklink.exe` on first start. Edit it and restart DeskLink.

```json
{
  "port": 8765,
  "groups": [
    {
      "title": "Media",
      "style": "media",
      "buttons": [
        { "id": "prev", "label": "Previous",   "icon": "prev",       "action": { "type": "keys", "keys": ["media_prev"] } },
        { "id": "play", "label": "Play/Pause", "icon": "play-pause", "action": { "type": "keys", "keys": ["media_play_pause"] } },
        { "id": "next", "label": "Next",       "icon": "next",       "action": { "type": "keys", "keys": ["media_next"] } },
        { "id": "mute", "label": "Mute",       "icon": "mute",       "action": { "type": "keys", "keys": ["volume_mute"] } }
      ]
    },
    {
      "title": "Quick",
      "style": "quick",
      "buttons": [
        { "id": "q_cs2",  "label": "Counter-Strike 2", "icon": "gamepad", "action": { "type": "game", "target": "cs2" } },
        { "id": "obs",    "label": "OBS",     "icon": "monitor", "action": { "type": "launch", "target": "C:\\Program Files\\obs-studio\\bin\\64bit\\obs64.exe" } },
        { "id": "sleep",  "label": "Sleep",   "icon": "moon",    "confirm": true, "action": { "type": "system", "command": "sleep" } },
        { "id": "lock",   "label": "Lock PC", "icon": "lock",    "confirm": true, "action": { "type": "system", "command": "lock" } }
      ]
    }
  ],
  "games": [
    { "id": "cs2",    "name": "Counter-Strike 2", "steam_appid": 730 },
    { "id": "mygame", "name": "My Game",          "target": "D:\\Games\\MyGame\\MyGame.exe" },
    { "id": "viaapp", "name": "Launcher Game",    "target": "C:\\Launcher\\launcher.exe", "args": "--launch mygame",
      "icon_from": "C:\\Games\\MyGame\\game.exe" }
  ]
}
```

## Group styles

| Style | Layout |
|---|---|
| `media` | Now-playing card. Buttons 1–3 are previous / play-pause / next; an optional 4th (mute) sits next to the volume slider. |
| `quick` | Game buttons as a row of square icons, other buttons as a row of icons, and CPU/GPU rings below. The time and date sit at the top. |
| `list` | One button per row. |
| `grid` | Square tiles. |

## Actions

| Type | Fields | What it does |
|---|---|---|
| `launch` | `target`, optional `args` | Opens an app, file, folder or URI (`spotify:`, `https://…`) as if double-clicked. `%VAR%` is expanded and programs start in their own folder. |
| `keys` | `keys`: 1–4 key names | Presses a key combination. |
| `system` | `command`: `lock` or `sleep` | Locks the PC or puts it to sleep. |
| `game` | `target`: a game `id` | Launches one of your `games`. |
| `page` | `target`: `home`, `games`, `stats` or `clock` | Switches the phone to another screen. |

Set `"confirm": true` on a button to ask before it runs.

**Key names:** `ctrl` `shift` `alt` `win`, `a`–`z`, `0`–`9`, `f1`–`f24`, `enter` `esc` `tab` `space` `backspace` `delete`, `up` `down` `left` `right` `home` `end` `pageup` `pagedown` `printscreen`, `volume_up` `volume_down` `volume_mute`, `media_play_pause` `media_next` `media_prev` `media_stop`.

**Icons:** `music` `play-pause` `prev` `next` `volume-down` `volume-up` `mute` `screenshot` `lock` `moon` `gamepad` `folder` `monitor` `mic` `mic-off` `globe` `terminal` `power`. Any other value is shown as text, so an emoji works too.

## Games

The easiest way to add games is the dashboard: open it from the tray icon, go to **Games** and choose **Add game**. You can paste a path (quotes from Explorer's *Copy as path* are fine), browse for the program, or pick an installed Steam game. Changes are saved to `config.json` and show up on connected phones right away. Everything below can also be set by hand.

- **Steam:** set `steam_appid` (the number in the game's store URL). DeskLink launches it through Steam, uses the cover art from your local Steam library, and takes the icon from the game's main executable.
- **Anything else:** set `target` to the game's `.exe` (plus `args` if needed). If the game is started through a launcher, set `icon_from` to the game's own executable so the tile shows the right icon.
- **Custom art:** set `cover` to a JPEG or PNG file.

DeskLink checks the whole config at startup and tells you exactly what is wrong if something does not add up.

## Troubleshooting

**The phone cannot open the page.** Make sure the phone is on the same Wi-Fi as the PC (not mobile data or a guest network), and type the full address including `http://` and the port. If Windows Firewall blocked DeskLink, allow it for private networks in *Windows Security → Firewall → Allow an app*.

**The address changes after a restart.** Give the PC a fixed IP in your router settings (often called a DHCP reservation).

**The phone screen turns off.** iOS does not let web pages keep the screen awake. Set *Settings → Display & Brightness → Auto-Lock* to *Never* while the phone is on your desk.

**A game does not start.** Check that `target` points to the same file your desktop shortcut uses. Many games need to start from their own folder, which DeskLink does automatically.

## Building from source

Requires Go 1.23 or later.

```powershell
go test ./...
go build -trimpath -ldflags "-s -w -H windowsgui" -o dist\desklink.exe ./cmd/desklink
```

`-H windowsgui` builds the tray app without a console window. Leave it out, or use `go run ./cmd/desklink`, to see the log in a terminal while developing. The log is also written to `desklink.log` next to `config.json`.

How the pieces fit together: [ARCHITECTURE.md](ARCHITECTURE.md).
