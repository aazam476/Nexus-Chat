# Permissions

Who may do what in a server (spec §4.4). One module decides it:
[`apps/server/internal/permissions`](../apps/server/internal/permissions).
Every check in the server goes through it, and clients use the result the
server sends them; nothing else resolves permissions.

## The permissions

| Permission                 | Lets a member                                                               |
| -------------------------- | --------------------------------------------------------------------------- |
| `ADMINISTRATOR`            | Do everything, in every channel, whatever the overrides say                 |
| `MANAGE_SERVER`            | Change the server's name, icon, description and settings                    |
| `MANAGE_ROLES`             | Create, edit, order and assign roles below their own highest role           |
| `MANAGE_CHANNELS`          | Create, edit, order and delete channels and categories, and their overrides |
| `MANAGE_EMOJI`             | Add and remove the server's custom emoji                                    |
| `VIEW_AUDIT_LOG`           | Read the server's audit log                                                 |
| `CREATE_INVITES`           | Make invite links                                                           |
| `KICK_MEMBERS`             | Remove members below their own highest role                                 |
| `BAN_MEMBERS`              | Ban members below their own highest role                                    |
| `TIMEOUT_MEMBERS`          | Time out members below their own highest role                               |
| `VIEW_CHANNEL`             | See a channel at all; without it, nothing else in that channel counts       |
| `SEND_MESSAGES`            | Post in a text channel                                                      |
| `SEND_IN_THREADS`          | Post in threads                                                             |
| `CREATE_THREADS`           | Start threads                                                               |
| `READ_HISTORY_BEFORE_JOIN` | Read messages sent before they could see the channel                        |
| `ATTACH_FILES`             | Send attachments                                                            |
| `ADD_REACTIONS`            | React to messages                                                           |
| `USE_CUSTOM_EMOJI`         | Use the server's custom emoji                                               |
| `MENTION_EVERYONE`         | Mention `@everyone`, `@here` and roles                                      |
| `MANAGE_MESSAGES`          | Delete anyone's messages                                                    |
| `CONNECT_VOICE`            | Join a voice channel                                                        |
| `SPEAK`                    | Talk in a voice channel                                                     |
| `VIDEO`                    | Turn on their camera in a voice channel                                     |
| `SCREEN_SHARE`             | Share their screen in a voice channel                                       |
| `MUTE_MEMBERS`             | Mute others in a voice channel                                              |
| `DEAFEN_MEMBERS`           | Deafen others in a voice channel                                            |
| `MOVE_MEMBERS`             | Move others between voice channels                                          |

Each permission is one bit; the stored values never change meaning, and
new permissions are added at the end.

A new server's `@everyone` role may view channels, send messages, use
threads, read history from before they joined, attach files, react, use
custom emoji, create invites, and join, speak, share video and share their
screen in voice channels. Everything that manages the server or other
members starts off.

## How they are worked out

**In the server** (outside any channel):

1. The server's owner may do everything.
2. Otherwise, start with the `@everyone` role's permissions and add every
   permission of every role the member has.
3. If that includes `ADMINISTRATOR`, the member may do everything.
4. A member who is timed out keeps only `VIEW_CHANNEL` and
   `READ_HISTORY_BEFORE_JOIN` until the timeout ends.

**In a channel**, start from steps 1–3, then let the channel's overrides
change the result, from the most general to the most specific. An
override can allow a permission, deny it, or say nothing about it.

5. The channel's override for `@everyone`: its denies come off, then its
   allows go on.
6. The overrides for all of the member's roles, taken together: every deny
   of every role comes off, then every allow of every role goes on. So if
   one of the member's roles allows something and another denies it, the
   allow wins, and the order of the roles does not matter.
7. The override for the member themselves: denies off, then allows on.
   This is the most specific, so it has the last word.
8. Without `VIEW_CHANNEL`, the member can do nothing in the channel.
9. A timed-out member keeps only `VIEW_CHANNEL` and
   `READ_HISTORY_BEFORE_JOIN`, as in step 4.

The owner and administrators skip steps 5–9: no override applies to them.

Managing other members and roles also follows the role order: a member may
kick, ban, time out, or change the roles of someone only if their own
highest role is above that person's highest role, and may only manage
roles below their own highest. The owner may manage anyone.

## A worked example

A server has two roles besides `@everyone`:

- `@everyone`: the defaults above.
- **Moderators**: `KICK_MEMBERS`, `TIMEOUT_MEMBERS` and `MANAGE_MESSAGES`.

Dana is a Moderator; Sam has no roles. In the server:

- Dana may do everything `@everyone` may, plus kick, time out, and delete
  anyone's messages (step 2).
- Sam may do what `@everyone` may.

The `#announcements` channel has two overrides:

- `@everyone`: deny `SEND_MESSAGES` and `ADD_REACTIONS`.
- Moderators: allow `SEND_MESSAGES`.

In `#announcements`:

| Step                    | Dana                                                      | Sam                                   |
| ----------------------- | --------------------------------------------------------- | ------------------------------------- |
| 1–3, server             | defaults + kick, time out, manage messages                | defaults                              |
| 5, `@everyone` override | loses send messages and add reactions                     | loses send messages and add reactions |
| 6, role overrides       | Moderators allow send messages: she gets it back          | no roles, no change                   |
| 7, member override      | none                                                      | none                                  |
| 8, view channel?        | yes                                                       | yes                                   |
| Result                  | may read, post and delete messages, but not add reactions | may read, but not post or react       |

If Sam is timed out, he keeps only viewing channels and reading history,
everywhere, until the timeout ends.

To hide a channel from everyone except Moderators, deny `VIEW_CHANNEL` for
`@everyone` and allow it for Moderators. A member override that denies
`VIEW_CHANNEL` hides it from that one member even if one of their roles
allows it.

## Who enforces them

The server enforces everything it can see: server settings, roles,
channels, members, invites, who receives a channel's messages, and who may
send to a channel or thread at all. What happens inside an end-to-end
encrypted message is hidden from it (spec §5.2): reactions, mentions,
custom emoji in text, attachments and deletions of others' messages. For
those, every member's app checks the sender's permissions, which the
server sends it, and ignores what the sender was not allowed to do. The
result is the same for everyone who follows the rules, and a modified app
cannot make other members' apps show what it was not allowed to send.

Managing follows three more rules, all enforced by the server:

- Nobody allows or denies, in a role or a channel override, a permission
  they do not have themselves, unless they are an administrator or the
  owner.
- Roles, and members' roles, can only be changed below the actor's own
  highest role. `@everyone` is never given or taken, and its name is fixed.
- Rearranging channels lists every channel, so it needs `VIEW_CHANNEL` on
  every one of them as well as `MANAGE_CHANNELS`.

`VIEW_CHANNEL` also decides who is in a text channel's MLS group
([e2ee-design.md](e2ee-design.md) §3.1), so the server works the groups out
again after every change. `READ_HISTORY_BEFORE_JOIN` is enforced mainly by
members' apps, which share history keys with a newcomer only when the
channel grants it. The server adds a coarser limit: without the permission,
a channel's archive is only served from the time the member joined the
server.

## Tests

`resolve_test.go` checks every combination of the layers that can decide a
permission (owner, administrator role, `@everyone` and role permissions,
timeout, and the `@everyone`, two role and member overrides: 1,944 cases)
against the rule above written the other way round, from the most specific
layer down. Property tests check that the order of roles and overrides
never matters, that adding a role or an allow never takes a permission
away, that a member's own deny always wins for everyone but administrators
and the owner, and that no unknown bit ever comes out. The worked example
above is a test too.
