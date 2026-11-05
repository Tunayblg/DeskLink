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
        { "id": "lock",   "label": "Lock PC", "icon": "lock",    "confirm": true, "action": { "type": "system", "command": "lock" } },
        { "id": "shutdown", "label": "Shut down", "icon": "power", "confirm": true, "action": { "type": "system", "command": "shutdown" } }
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
| `system` | `command`: `lock`, `sleep` or `shutdown` | Locks the PC, puts it to sleep, or shuts it down (a program with unsaved work can still hold the shutdown back). Give shutdown `"confirm": true`. |
| `game` | `target`: a game `id` | Launches one of your `games`. |
| `page` | `target`: `home`, `games`, `stats` or `clock` | Switches the phone to another screen. |

Set `"confirm": true` on a button to ask before it runs.

**Key names:** `ctrl` `shift` `alt` `win`, `a`–`z`, `0`–`9`, `f1`–`f24`, `enter` `esc` `tab` `space` `backspace` `delete`, `up` `down` `left` `right` `home` `end` `pageup` `pagedown` `printscreen`, `volume_up` `volume_down` `volume_mute`, `media_play_pause` `media_next` `media_prev` `media_stop`.

**Icons:** `music` `play-pause` `prev` `next` `volume-down` `volume-up` `mute` `screenshot` `lock` `moon` `gamepad` `folder` `monitor` `mic` `mic-off` `globe` `terminal` `power`. Any other value is shown as text, so an emoji works too.

## Games

The easiest way to add games is the dashboard: open it from the tray icon, go to **Games** and choose **Add game**. You can paste a path (quotes from Explorer's *Copy as path* are fine), browse for the program, or pick an installed Steam game. Changes are saved to `config.json` and show up on connected phones right away. Everything below can also be set by hand.

- **Steam:** set `steam_appid` (the number in the game's store URL). DeskLink launches it through Steam, uses the cover art from your local Steam library, and takes the icon from the game's main executable.
- **Anything else:** set `target` to the game's `.exe` (plus `args` if needed). If the game is started through a launcher, set `icon_from` to the game's own executable so the tile shows the right icon.
- **A shortcut:** `target` can also be a `.lnk` shortcut (for example the one behind a pinned taskbar icon, which is in `%AppData%\Microsoft\Internet Explorer\Quick Launch\User Pinned\TaskBar`). Set `icon_from` to the game's real `.exe` so the tile gets its icon.
- **Custom art:** set `cover` to a JPEG or PNG file.

On the phone a game starts with a **double tap**; a single tap only shows a hint. If the game's program is already running, DeskLink does not start it again and the phone says so. DeskLink knows the program from `icon_from`, or from `target` when that is an `.exe`; a game with neither (a bare shortcut) can always be started, and Steam games are left to Steam.

DeskLink checks the whole config at startup and tells you exactly what is wrong if something does not add up.

## Home screen tiles

The home screen has six tiles, in two rows of three. Each one opens a game, a website or a program on the PC; an empty slot shows a **+**. The easiest way to fill them is the dashboard (**Games → Home screen**, with Netflix, YouTube and more ready to pick), but they live in `config.json` too:

```json
"shortcuts": [
  { "game": "hades" },
  { "name": "Netflix", "target": "https://www.netflix.com", "tint": "229,9,20" },
  {},
  { "name": "Notepad", "target": "C:\\Windows\\notepad.exe" }
]
```

`game` is the `id` of one of your games (it opens with the launch animation). Otherwise `target` is a web address (`http://` or `https://`) or a program, file or URI, and `name` is required. `tint` is an optional tile color as `r,g,b`. Games and programs show their own icon. Well-known services (Netflix, YouTube, Twitch, Spotify, Prime Video, Disney+) are recognized from their address and get a small icon of their own, in their own color unless you set `tint`; any other website shows its first letter on the tint color. There are at most six entries; an empty `{}` leaves the slot empty.

Older configs that pinned games to the home screen are converted automatically the first time this version starts: those games become the first tiles.

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
