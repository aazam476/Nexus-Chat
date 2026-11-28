# Moderation

Spec §4.7. This page describes what server moderators and instance admins
can do, and what the server records about it.

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
