# DeskLink for Android

A small app that opens the DeskLink panel full screen. Android browsers only install web apps from HTTPS sites, and DeskLink runs over plain HTTP on your home network, so a browser shortcut would always keep the address bar. The app has none, keeps the screen on while it is open, and remembers your PC.

On first start it asks for the PC: scan the QR code from the dashboard (with Google's code scanner, so the app needs no camera permission) or type the address. The Back button opens that screen again, to scan a new code, switch PCs or exit.

## Build

The CI workflow builds `DeskLink.apk` on every push (see `.github/workflows/ci.yml`). To build it yourself you need JDK 17, the Android SDK (platform 34) and Gradle 8.9, or simply open this folder in Android Studio:

```
gradle -p android assembleRelease
```

## Signing

Android only updates an installed app with a new APK signed by the same key, so releases are signed with one fixed key. CI reads it from three repository secrets:

| Secret | Value |
|---|---|
| `DESKLINK_KEYSTORE_BASE64` | The PKCS#12 key store, base64-encoded |
| `DESKLINK_KEYSTORE_PASSWORD` | Its password |
| `DESKLINK_KEY_ALIAS` | The key's alias |

Without them (for example on a pull request from a fork), the build is signed with the debug key instead. That is fine for trying the app, but such a copy cannot later be updated by a release build.
