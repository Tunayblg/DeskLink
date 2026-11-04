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
| A website open in the PC's browser uses the dashboard | `/admin` only answers loopback requests addressed to `localhost` or `127.0.0.1` (this also defeats DNS rebinding), and its API needs a random per-run token that exists only inside the dashboard page. Other sites cannot read that page. |
| A stolen `devices.json` | Only SHA-256 hashes of tokens are stored. They cannot be used to connect. |
| A compromised or malicious phone runs arbitrary code | The protocol only carries button IDs and a volume level (0–100). Actions are defined in `config.json` on the PC and validated at startup. There is no shell or command action. |
| Flooding a button | Repeated presses of the same button within 120 ms are ignored, and messages are limited to 4 KB. |

## Limitations

- Traffic is plain HTTP and WebSocket on your local network. Someone who can capture traffic on your Wi-Fi could see button presses and a device token and replay it. Use DeskLink on networks you trust, such as your home Wi-Fi.
- If a phone is lost, remove it in the dashboard. Its token stops working immediately.

## Reporting a vulnerability

Please open a private security advisory on GitHub instead of a public issue.
