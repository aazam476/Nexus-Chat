# ADR 0009: Voice transport modes and Auto switching

- Status: **Proposed [REVIEW]**
- Date: 2026-11-07

## Context

Admins choose SFU (LiveKit), P2P mesh (cap 2–8, default 6), or Auto
(default: P2P up to the cap, then SFU). Media is E2EE in every mode, with
keys from the conversation's MLS group.

## Considerations

- In a mesh, each participant uploads one stream per peer. With video at
  ~1.5 Mbit/s, a 6-person mesh needs ~7.5 Mbit/s upload per participant,
  more than many home and mobile connections can sustain. Audio-only
  (~40 kbit/s per stream) is fine at 8.
- Switching transports mid-call interrupts media briefly (new connections,
  ICE negotiation). Switching back and forth is worse than staying on SFU.

## Decision (proposed)

- **Threshold:** the admin's cap counts **participants**. In addition, Auto
  mode moves to SFU when a **4th participant publishes video or screen
  share** even if the cap has not been reached, because upload cost, not
  headcount, is what breaks a mesh. Audio-only calls stay P2P up to the cap.
- **Switching up (P2P → SFU):** triggered by the server when a join or a
  new video/screen track would cross the threshold. Every client connects to
  the LiveKit room first, starts publishing there, then closes the mesh
  ("make before break"). Media keys stay the same because they come from MLS,
  not the transport. Target gap: under 1 second of audio.
- **No switching down during a call.** Once on SFU, a call stays there until
  everyone leaves. This avoids flapping when people drop in and out.
- **Coturn missing:** if P2P or Auto is selected and the server's TURN health
  check fails, the Admin panel shows a warning and the server hands out SFU
  only.
- **DMs (two people):** P2P whenever P2P or Auto is enabled.

## Consequences

- Auto mode always needs LiveKit running, even if most calls are small.
- The switching path gets its own Playwright test (third participant turns
  on video → call moves to SFU with no lost audio beyond the target gap).
