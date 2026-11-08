# Nexus Chat — Platform Support

Minimum versions: Chrome, Edge, Firefox and Safari (latest two); Windows 10+,
macOS 13+, Ubuntu 22.04+ (Electron); iOS 16+, Android 10+.

This file lists every feature that cannot work, or works differently, on a
platform. It starts with the Phase 0 research and is updated in each milestone.

## 1. Mac App Store sandbox **[REVIEW]**

Status: **researched from documentation, not yet confirmed on hardware.**
Confirming needs a signed MAS build on a Mac. That happens as the first task of
milestone M11. The table gives the expected result and the fallback if the
spike disagrees.

| Feature                                  | Mechanism                                                                                                          | Expected in sandbox                                                 | Evidence                                                                                                                                                       | Fallback                                                                                                              |
| ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| Global hotkey (press)                    | Electron `globalShortcut` (Carbon hot keys)                                                                        | Works, no extra permission                                          | Electron docs; used by sandboxed apps                                                                                                                          | —                                                                                                                     |
| Push-to-talk **release** while unfocused | `globalShortcut` reports key-down only. Detecting key-up needs a listen-only `CGEventTap` in a small native module | Works after the user grants **Input Monitoring** in System Settings | Apple DTS (Developer Forums thread 780626) says sandboxed apps may use Input Monitoring through `CGEventTap`; Accessibility is not available to sandboxed apps | Push-to-talk only while the app is focused, plus a "toggle-to-talk" global hotkey that works without Input Monitoring |
| Screen / window picker                   | Electron `desktopCapturer` (ScreenCaptureKit on macOS 13+)                                                         | Works after the user grants **Screen Recording**                    | ScreenCaptureKit is the system API for sandboxed capture                                                                                                       | —                                                                                                                     |
| System audio in screen share             | `desktopCapturer` loopback audio via ScreenCaptureKit                                                              | Uncertain in MAS builds                                             | Electron documents loopback audio as ScreenCaptureKit-only; no source found confirming it in MAS builds                                                        | Screen share without system audio on macOS, as the spec already allows ("where the OS allows")                        |
| Camera / microphone                      | `com.apple.security.device.camera` / `.audio-input` entitlements                                                   | Works                                                               | Electron MAS guide                                                                                                                                             | —                                                                                                                     |
| Tray icon                                | `Tray`                                                                                                             | Works                                                               | —                                                                                                                                                              | —                                                                                                                     |
| Keychain (`safeStorage`)                 | App's own keychain access group                                                                                    | Works                                                               | —                                                                                                                                                              | —                                                                                                                     |
| Push notifications                       | APNs for macOS via the relay                                                                                       | Works with the push entitlement                                     | —                                                                                                                                                              | Web Push while the app runs                                                                                           |

The Electron MAS guide also warns that "video capture may not work for some
machines" in MAS builds. The spike tests on Apple silicon and Intel.

Sections 2–4 come from platform documentation. Each row is checked again in
the milestone that builds the feature.

## 2. Web

| Feature                                    | Limit                                                                                                         |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------- |
| Everything E2EE, voice, passkeys, Web Push | Needs HTTPS or `localhost`                                                                                    |
| Screen share audio                         | Chromium: tab audio (and system audio on Windows when sharing the whole screen). Firefox and Safari: no audio |
| Push-to-talk                               | Only while the tab is focused                                                                                 |
| Link previews                              | Only through the optional instance proxy                                                                      |
| Web Push on iOS Safari                     | Only when the site is added to the Home Screen (iOS 16.4+)                                                    |
| Window picker                              | Browser-provided picker only                                                                                  |

## 3. Mobile

| Feature                      | iOS                                                                                                                  | Android                                                                            |
| ---------------------------- | -------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Screen share                 | Broadcast upload extension (whole screen only)                                                                       | MediaProjection (whole screen; app-only on Android 14+ where the system offers it) |
| System audio in screen share | Captured by the broadcast extension (app audio only, per iOS rules)                                                  | Android 10+ `AudioPlaybackCapture`, only from apps that allow capture              |
| Push-to-talk                 | On-screen button only                                                                                                | On-screen button only                                                              |
| Fully self-hosted push       | Not possible with the store app ([`push-notifications.md`](push-notifications.md))                                   | UnifiedPush                                                                        |
| Passkeys                     | Not yet: each instance must publish `apple-app-site-association` naming the app, planned with the native work in M11 | Not yet: each instance must publish `assetlinks.json`, planned for M11             |

## 4. Windows and Linux desktop

| Feature                           | Limit                                                                                    |
| --------------------------------- | ---------------------------------------------------------------------------------------- |
| System audio (Windows)            | Supported via loopback capture                                                           |
| System audio (Linux)              | Through PipeWire screen-cast portal when available; otherwise none                       |
| Global hotkeys on Linux (Wayland) | Through the GlobalShortcuts portal where the desktop supports it; otherwise focused-only |
| Flatpak sandbox                   | Screen capture through the xdg-desktop-portal                                            |
