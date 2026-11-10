<div align="center">

<img src="web/static/icon-180.png" width="84" alt="DeskLink icon">

# DeskLink

**Your old phone, reborn as a Stream Deck for your PC.**

[![CI](https://github.com/Tunayblg/DeskLink/actions/workflows/ci.yml/badge.svg)](https://github.com/Tunayblg/DeskLink/actions/workflows/ci.yml)
![Platform](https://img.shields.io/badge/platform-Windows%2010%20%7C%2011-0078D4)
[![Download](https://img.shields.io/github/v/release/Tunayblg/DeskLink?label=download&color=38bdf8)](https://github.com/Tunayblg/DeskLink/releases/latest)

<img src="docs/images/hero.png" alt="DeskLink on a phone" width="100%">

</div>

Media controls with cover art, a game launcher, per-app volume, screen brightness, mic mute and live CPU/GPU stats, on the phone that is already in your drawer.

- 📱 **No app to install on iPhone.** Open it in Safari and add it to the home screen. Android gets a small app for true full screen.
- 🪶 **Featherweight.** One small `.exe`, roughly 20–50 MB of RAM and almost no CPU.
- 🎮 **Made for gaming.** Launch a game and the phone turns into a live hardware monitor.
- 🍿 **Watch mode.** PC volume, play/pause, jump back and forward, screen brightness and a shut down button, made for the couch.
- 🧑‍💻 **Developer mode.** Dictate or write a prompt on the phone and it is typed into Claude Code, Codex or any tool on your PC, with quick keys (Esc, Tab, arrows, Ctrl+C, Enter).
- ⭐ **Six home screen tiles.** Each opens a game, Netflix, YouTube, any website or any program.
- 🎚️ **More than buttons.** A volume mixer for each program, one-tap mic mute and a trackpad for your PC.
- 🔒 **Private.** Stays on your Wi-Fi. No cloud, no account. Pair by scanning a QR code.
- 🖥️ **Easy to manage.** A desktop dashboard for games, tiles and phones, with a built-in illustrated guide for iPhone and Android.
- 📲 **Phones and tablets.** The layout adapts to bigger screens, and it runs on old devices too (built and tested on an iPhone 6s).

## Screens

<table>
  <tr>
    <td width="50%"><img src="docs/images/home.png" alt="Home"><br><b>Now playing, six tiles and quick actions</b></td>
    <td width="50%"><img src="docs/images/watch.png" alt="Watch mode"><br><b>Watch mode</b> for the couch: volume, transport, brightness</td>
  </tr>
  <tr>
    <td><img src="docs/images/stats.png" alt="Game mode"><br><b>Game mode</b>: live stats and a mixer</td>
    <td><img src="docs/images/games.png" alt="Games"><br><b>Your games</b>, one tap away</td>
  </tr>
  <tr>
    <td><img src="docs/images/launch.png" alt="Launch"><br><b>Launch animation</b></td>
    <td><img src="docs/images/clock.png" alt="Clock"><br><b>Desk clock</b>, a rev-counter dial with shift lights for the seconds</td>
  </tr>
</table>

## Manage it from your PC

A clean desktop dashboard lives in the tray. Add a phone by scanning a QR code, see who is connected, and set up your games and home screen tiles without touching a config file.

<table>
  <tr>
    <td width="50%"><img src="docs/images/dashboard-games.png" alt="Games and home screen tiles"><br><b>Games and home screen tiles</b>, shown on the phone instantly</td>
    <td width="50%"><img src="docs/images/dashboard-tile.png" alt="Picking a tile"><br><b>Pick a tile</b>: a game, Netflix, YouTube, any address or program</td>
  </tr>
  <tr>
    <td><img src="docs/images/dashboard-pair.png" alt="Pairing with a QR code"><br><b>Pair with a QR code</b>, the code changes every 30 s</td>
    <td><img src="docs/images/dashboard-guide.png" alt="The built-in guide"><br><b>A built-in guide</b>, a flip book for iPhone and one for Android</td>
  </tr>
</table>

## Get started

1. Download **[desklink.exe](https://github.com/Tunayblg/DeskLink/releases/latest)** and run it.
2. Scan the QR code in the dashboard with your phone's camera (same Wi-Fi).
3. **iPhone:** add it to the home screen from Safari. **Android:** install **DeskLink.apk** from the same page. It scans the QR code, or takes the address by hand.
4. Done. Right-click the tray icon to have DeskLink start with Windows.

The dashboard's **Guide** walks through every step, with the address and firewall details if a phone cannot connect.

> Windows may warn about the download because the app is not code-signed. Choose **More info → Run anyway**. If Smart App Control blocks it, it has to be turned off in *Windows Security → App & browser control* (it cannot be turned back on later).

Want your own buttons and layouts? See the **[configuration guide](docs/CONFIG.md)**. How it works and how it is secured: **[architecture](docs/ARCHITECTURE.md)** and **[security model](docs/SECURITY.md)**.

<sub>Windows 10/11 · any modern phone or tablet browser · GPU stats need an NVIDIA card</sub>
