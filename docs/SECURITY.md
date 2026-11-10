# Security model

DeskLink runs actions on your PC, so it assumes the local network is **not** fully trusted.

## Threats and mitigations

| Threat | Mitigation |
|---|---|
| A device on the internet reaches the port (e.g. through port forwarding) | Requests from non-private, non-loopback IPs are rejected with 403. Do not port-forward DeskLink. |
| Another device on the same Wi-Fi sends commands | Commands require a device token, which you only get with the pairing code shown on the PC (typed, or scanned as a QR code). |
| Guessing the pairing code | The code changes every 30 seconds and is derived from a random secret that exists only in memory. After 5 wrong codes, pairing is refused until the next code appears. "New code" in the dashboard invalidates the code on screen at once. |
| Reusing a pairing link | The QR code carries the code in the URL fragment, which is never sent to a server and expires with the code. The handoff code that lets a new iOS home screen app reuse a Safari pairing works once and expires after 24 hours. |
| A website open on your phone talks to DeskLink (cross-site WebSocket hijacking) | The WebSocket handshake rejects any `Origin` that does not match the host. |
| A website open in the PC's browser uses the dashboard | `/admin` only answers loopback requests addressed to `localhost` or `127.0.0.1` (this also defeats DNS rebinding), and its API needs a random per-run token that exists only inside the dashboard page. Other sites cannot read that page, and its Content Security Policy only runs scripts carrying a per-request nonce. |
| A stolen `devices.json` | Only SHA-256 hashes of tokens are stored. They cannot be used to connect. |
| A compromised or malicious phone runs arbitrary code | The protocol only carries button and tile IDs, volume and brightness levels (0–100), trackpad moves, one of three watch mode buttons, and, in developer mode, text (at most 2000 characters, control characters removed) plus seven fixed keys (Esc, Tab, Shift+Tab, the up and down arrows, Ctrl+C, Enter). What a button or tile opens is set on the PC, in `config.json` or the dashboard, and validated before it is used. There is no shell or command action. Watch mode can only press play/pause and the two arrow keys. Developer mode types text only into Claude, Codex, ChatGPT, terminal and editor windows (browser tabs count when the site name is a whole part of the title), like a keyboard would: either the window in front, if it is one of those, or one chosen from their list (their titles are sent to the phone; other windows are neither listed nor accepted; DeskLink checks the choice against a fresh list, brings the window forward and types only if it is really in front, and presses Enter only if it still is). In a terminal, line breaks become spaces so a dictated text never runs a line on its own. Live typing also presses Backspace to take back its own words; it counts only what it typed itself, stops if another window comes to the front, and only the newest phone may type live at a time; it cannot press other keys, and the tab can be hidden in the phone's settings (revoking the phone on the PC stops it for good). |
| A phone makes the screen unreadable | Software dimming stops at 10 % brightness, so the screen never goes black, and the dimming window disappears as soon as DeskLink quits. |
| Flooding | Repeated presses of the same button within 120 ms are ignored, messages are limited to 12 KB and rate limited per connection, and each phone has at most one brightness question in flight. |
| A web page in the Android app calls into the app, or the app is pointed at the internet | The app keeps two WebViews. Only its own connection screen, loaded from the app itself, has a bridge to the app, and that screen cannot navigate anywhere else. The panel loaded from the PC has no bridge and can only navigate within the PC's own address. The app only connects to private addresses (192.168.x.x, 10.x.x.x, 172.16–31.x.x, link-local) and plain host names, the same boundary the PC enforces. |

## Limitations

- Traffic is plain HTTP and WebSocket on your local network. Someone who can capture traffic on your Wi-Fi could see button presses and a device token and replay it. Use DeskLink on networks you trust, such as your home Wi-Fi.
- If a phone is lost, remove it in the dashboard. Its token stops working immediately.
- Releases are not code-signed, so Windows SmartScreen or Smart App Control may warn about them, and the Android app is installed from outside the Play Store.

## Reporting a vulnerability

Please open a private security advisory on GitHub instead of a public issue.
