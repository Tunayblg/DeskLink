# Security model

DeskLink runs actions on your PC, so it assumes the local network is **not** fully trusted.

## Threats and mitigations

| Threat | Mitigation |
|---|---|
| A device on the internet reaches the port (e.g. through port forwarding) | Requests from non-private, non-loopback IPs are rejected with 403. Do not port-forward DeskLink. |
| Another device on the same Wi-Fi sends commands | Commands require a device token, which you only get by entering the PIN shown on the PC. |
| Brute-forcing the PIN | The PIN changes after 5 wrong attempts and after every successful pairing. |
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
