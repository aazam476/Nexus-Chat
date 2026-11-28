# Moderation

Spec §4.7. This page describes what server moderators and instance admins
can do, and what the server records about it.

## Kick, ban and timeout

Each needs its permission (`KICK_MEMBERS`, `BAN_MEMBERS`, `TIMEOUT_MEMBERS`)
and a highest role above the target's; nobody can act on the owner or on
themselves, and the owner can act on anyone else.

- **Kick** takes a member out of the server. They can join again with an
  invite.
- **Ban** takes them out if they are a member and keeps them from joining
  until a moderator lifts it. Someone who is not a member can be banned
  ahead of time. A ban can also delete what they sent to the server's
  channels in the last hour, day or up to 7 days: the server deletes the
  ciphertext (archive copies and any live copies still waiting) and signs a
  directive telling members' devices to drop their decrypted copies
  (docs/e2ee-design.md §3.2).
- **Timeout** lasts from 60 seconds to 28 days. A timed-out member keeps
  only `VIEW_CHANNEL` and `READ_HISTORY_BEFORE_JOIN`: they can read but not
  send, react, start threads, speak or join calls. It ends by itself, or a
  moderator lifts it. The member list shows when each timeout ends.

Kicks and bans take the member out of the channels' MLS groups the same way
leaving does. A reason of up to 512 characters can be given; it goes in the
audit log.

## Deleting messages

Members with `MANAGE_MESSAGES` in a channel delete anyone's message there
(`DELETE /api/v1/servers/{id}/channels/{channelId}/messages/{messageId}`).
The server deletes its ciphertext, uncounts it from its thread, and signs a
directive so members' devices drop it. Senders delete their own messages
with an encrypted deletion instead, which needs no permission.

## Reports

Anyone can report a message they can read (`POST /api/v1/reports`), once.
The reporter's device opens the message's archive copy and hands over what
it proves (ADR 0010): the plaintext, and the sender's signature over it, its
channel or conversation, its message ID and its epoch. The server checks
that the evidence names that message, then signs a **receipt** with the
instance key for the copy it stores: the conversation and channel, the
message ID, the sender's user ID and address (`@name@domain`), the epoch,
the SHA-256 of the archive copy and when it was sent.

A moderator's device checks every report itself before showing it as
verified:

1. the instance signed the receipt;
2. the evidence is about the message the receipt names, at the same place
   and epoch;
3. the sender's signature covers exactly this plaintext at that address;
4. the signing device's credential chains to the sender's published
   identity, and the sender is the user the receipt names.

Reworded messages fail (3), a reporter's own message passed off as someone
else's fails (4), evidence about one message with another's receipt fails
(2), and a made-up or altered receipt fails (1). The server never needs a
history key, and never reads messages nobody reported.

Reports about a server's channels go to members with `MANAGE_MESSAGES`
there, who resolve them with a note or pass them on to the instance admins.
Reports about direct and group messages go to the instance admins. An admin
setting sends every server's reports to the admins too. Resolving and
passing on are recorded in the server's audit log (`report.resolve`,
`report.escalate`).

## Audit log

Every moderation and configuration action in a server is recorded: who did
it, what kind of thing it was done to and which one, what changed, and when.
Members with `VIEW_AUDIT_LOG` read it (`GET /api/v1/servers/{id}/audit-log`),
newest first, 50 entries a page by default and at most 100.

| Action                                               | Recorded with                                        |
| ---------------------------------------------------- | ---------------------------------------------------- |
| `server.update`, `server.icon`                       | the new settings; whether an icon was set or removed |
| `channel.create`, `channel.update`, `channel.delete` | the channel's name, kind and topic                   |
| `channel.order`, `role.order`                        | nothing more                                         |
| `channel.override_set`, `channel.override_clear`     | the role or member, and what is allowed and denied   |
| `role.create`, `role.update`, `role.delete`          | the role's name and permissions                      |
| `member.role_give`, `member.role_take`               | the role                                             |
| `invite.create`, `invite.revoke`                     | the first four characters of the code                |
| `emoji.add`, `emoji.remove`                          | the emoji's name                                     |

The log can be filtered by who acted, by action (or every action of a kind:
`role` matches `role.create`, `role.update` and so on), and by target.
Entries hold no message content and only the start of invite codes, so
reading the log never lets anyone read messages or join with an invite.
Entries are deleted after 90 days, checked every hour.

## The instance admin panel

Instance admins manage the whole instance (`/api/v1/admin/...`):

- **Accounts.** Search by username, display name or email. **Suspending**
  an account ends its sessions (its devices disconnect at their next
  heartbeat) and refuses every way of signing in, with `account_suspended`,
  until an admin reinstates it. The refusal comes only after the first
  factor is right, so a wrong password learns nothing. Admins can make
  other accounts admins or stop them being one. Nobody can do either to
  their own account, and the instance always keeps one admin who is not
  suspended.
- **Servers.** Search by name, with each server's owner and size, and
  delete one with everything in it; its members are told.
- **Registration.** Open, by invite only, or closed (registration
  invites are managed as before).
- **Federation.** Every instance but the blocked ones, only the allowed
  ones, or none, with up to 1,000 domains on each list. The policy is
  stored now and applied when federation arrives (M10).
- **Reports.** Whether every server's reports also come to the admins.
- **Numbers.** Accounts (and suspended ones), accounts active in the last
  30 days, servers, direct and group conversations, messages in the last
  day, open reports, and stored attachments.
