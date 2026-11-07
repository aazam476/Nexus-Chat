# Nexus Chat — Threat Model

Status: Phase 0 design. This file is the source for the in-app page "What can
the server see?" and is published with every release. An independent audit is
planned before 1.0, not before beta.

## 1. Assets

| Asset | Where it exists in plaintext |
|---|---|
| Message bodies, edits, reactions, thread contents, link previews | Member devices only |
| Attachments and their thumbnails | Member devices only |
| Call audio, video, screen share | Participant devices only |
| Identity keys (UMK, USK, device keys), history keys, backup key | Devices; the server holds only encrypted backups |
| Passwords | Sent to the server over TLS at login and hashed with Argon2id ([ADR 0004](adr/0004-password-handling.md)); never stored in plaintext |
| Search index | Devices; synced copies are encrypted with the backup key |

## 2. What each party can see

### 2.1 Home instance and hosting instance (the "server")

**Can see (metadata required to function):**
- Accounts: username, display name, email, avatar, bio, status, custom
  status, login methods, sessions and device list, IP addresses of
  connections, last-seen times.
- Social graph: friends, friend requests, blocks, privacy settings.
- Structure: servers, channels, categories, roles, permissions, invites,
  bans, timeouts, audit log, custom emoji and server icons (public assets).
- Membership: who is in which server, channel audience, DM and group DM.
- Per message: sender, conversation, server timestamp, ciphertext sizes,
  which message it replies to or which thread it belongs to (needed for
  threading and fetching), **mention hints** (the user IDs a sender's client
  chose to reveal for notifications), and whether it has attachments, plus
  each attachment's size.
- Read markers and unread counts, typing indicators (in memory only),
  presence.
- Reactions: **not** visible. Reactions are encrypted messages, so the
  server sees only that a small message referencing another message was sent.
- Voice: who is in which call and when, mute/deafen state, and approximate
  media bitrate. Not the media itself.
- Search: nothing in on-device mode. If encrypted index sync is used, chunk
  sizes and which (channel, month) chunks a device downloads.
- Web link-preview proxy (optional, off by default): the URLs the web
  client asks it to fetch.
- Moderation reports: the reported plaintext, once a user chooses to report
  it.

**Cannot see:** message content, attachment content, reaction emoji, link
preview content, call media, keys, or the search index.

### 2.2 Official push relay

Can see: the device's APNs/FCM token, the sending instance's domain and key,
the time and size of each push, and the opaque handle. Cannot see: content
(the payload is the encrypted message or a fetch pointer), which channel or
DM it belongs to, or who sent it. Payloads are kept in RAM only and dropped
after delivery. See [`push-notifications.md`](push-notifications.md).

### 2.3 Apple and Google push services

Can see: device token, timing, size, and the ciphertext payload. Same as the
relay, minus the instance identity.

### 2.4 LiveKit SFU and TURN servers

Can see: participants' IP addresses, timing, packet sizes, and track
metadata (which participant publishes audio, video or screen). Cannot see:
frame contents (E2EE).

### 2.5 Federated instances

A remote hosting instance sees what any server sees for its own servers,
including remote members' addresses and activity in those servers. The user's
home instance additionally sees which remote servers the user belongs to.

### 2.6 Network attacker

Sees TLS-protected traffic only, given HTTPS (see §5 for plain HTTP).

## 3. Attacker models

| # | Attacker | Goal | Mitigations | Residual risk |
|---|---|---|---|---|
| A1 | Curious instance operator | Read messages | MLS + archive encryption; keys only on devices | Full metadata visibility (§2.1) |
| A2 | Malicious instance operator | Inject a device or fake member into a conversation | Device certificates chained to the user's UMK; identity-change warnings; optional verification | A user who never verifies and is attacked at first contact (TOFU) |
| A3 | Malicious operator | Serve a modified web client that steals keys | None technically complete for web. Native and desktop apps come from app stores and are not served by the instance | **Known limitation:** the web client trusts the instance that serves it. Documented; the native apps are recommended for high-risk users |
| A4 | Malicious operator | Withhold or reorder messages, split the group | MLS epoch and transcript hash detect forks; clients warn on missing messages using per-sender counters in the plaintext | Denial of service is always possible |
| A5 | Removed member | Read new messages | Epoch change on removal; server stops delivery | Window between removal and the next commit, during which the server already refuses delivery |
| A6 | Compromised device | Read past and future messages | MLS PCS heals live messages after the device is removed; device revocation | History keys on that device expose history it could read. Inherent to history features |
| A7 | Another instance (federation) | Impersonate users, spam | RFC 9421 signatures, home-instance authentication, allow/deny lists, rate limits | An open-federation instance can host abusive servers; users must join to be affected |
| A8 | Rogue instance using the push relay | Push to devices registered with other instances | Instance-bound handles, request signatures, rate limits, blocklist | An instance can still push to its own users |
| A9 | Stolen database or backup | Read content | Ciphertext only; Argon2id password hashes; backups need the user's recovery method | Offline password guessing against the Argon2id hash, and against the password-wrapped backup key for users who enable that method |
| A10 | Abusive member | Forge a report against someone | Reports need the sender's signature plus a server receipt ([ADR 0010](adr/0010-abuse-reports.md)) | — |

## 4. Known limitations (beta)

1. **Metadata is not hidden.** Membership, timing, sizes and the social
   graph are visible to the instance (§2.1).
2. **Web client code is served by the instance** (A3).
3. **Messages are not deniable.** MLS signs every message; reports rely on
   this.
4. **History sharing weakens forward secrecy for history** by design (see
   [ADR 0003](adr/0003-history-sharing.md)).
5. **Trust on first use** for contacts' identity keys unless verified.
6. **The login password reaches the server** (Argon2id, not OPAQUE) if
   [ADR 0004](adr/0004-password-handling.md) is accepted.
7. **No independent audit yet.** The chosen MLS library has no published
   formal audit ([ADR 0002](adr/0002-mls-library.md)).
8. **Mention hints** reveal who a message mentions.
9. **Plain-HTTP deployments** (§5).

## 5. Plain HTTP

Over plain HTTP (except `localhost`), the browser disables WebCrypto and
media capture, so the web client cannot run E2EE at all: it shows the HTTPS
notice and refuses to log in instead of falling back to sending plaintext.
Native apps refuse plain-HTTP instances. Setup tokens and passwords travel
unencrypted; the setup wizard warns about this.
