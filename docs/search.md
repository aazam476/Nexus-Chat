# Search

Message content is end-to-end encrypted, so the server cannot index it
(spec §4.6). Message search runs on the user's devices; the server only
searches what it already holds in plain form (names of servers, channels and
people). This page describes how the on-device search works.

## The index

Each device keeps an index of the messages it has decrypted, in its encrypted
database (ADR 0013), in **chunks**: one per conversation (or channel) and
month. A chunk holds, for each message, who sent it, when, whether it has
attachments, which thread it is in and its text, and for each word the
messages that contain it.

- **Words.** Text is folded (lowercase, accents and other combining marks
  removed, so "Café" finds "cafe") and split into runs of letters and digits
  in any script. Scripts written without spaces (Chinese, Japanese, Korean)
  are split into overlapping pairs of characters, so a word inside a sentence
  can still be found. Every query word must match the start of a word in the
  message: "rep" finds "report".
- **Filters.** From a person, in conversations or channels, with attachments,
  and between dates. A search reads only the chunks its filters allow.
- **Changes.** Edits replace a message's words and deletions take it out, by
  the same rule the conversation view uses: only the sender's own edits and
  deletions count. Reactions are not searchable.

The same chunk format is used on every platform. ADR 0013 expected SQLite FTS5
on phones; a chunk store on top of the encrypted record store was chosen
instead, because the chunks are also the unit of index sync (below) and work
the same in browsers, where there is no SQLite.

## Tiers

1. **Recent window.** A device indexes the last 90 days of every conversation
   and channel it keeps, as messages arrive or are read from the archive.
2. **Search older messages.** In a conversation's or channel's search, the
   user can ask to index its whole history. The device indexes everything it
   holds, then reads the archive further back a page at a time, saving where
   it got to after each page, so the work resumes after an interruption. On
   phones it waits for Wi-Fi and charging unless the user says to go ahead
   now; browsers and the desktop app go ahead at once.
3. **Index sync between the user's devices.** Every ten minutes while
   messaging runs (and five seconds after it starts), a device adds the
   chunks its other devices shared, then uploads the chunks it changed. Each
   chunk goes to the key backup as the blob `search.<conversation>.<month>`,
   sealed under the backup key with its name bound in. A new device fetches
   only the chunks from the recent window. The server sees opaque blobs,
   their names (which conversation and month) and their sizes, as listed in
   the threat model. Chunks over 900 KB are not shared (each device rebuilds
   them), and at most 5,000 chunks are kept, below the backup's 10,000-blob
   limit.
4. **Desktop as the heavy indexer.** The desktop app indexes the whole history
   of every conversation and channel it keeps, and shares those chunks, so
   phones can search old messages without downloading them.

Search depends on the history-key backup (docs/e2ee-design.md §5): a device
that cannot decrypt old messages cannot index them, but it can still use
chunks another of the user's devices shared.

## Names, on the server

Names of servers, channels and people are not end-to-end encrypted, so the
server searches them (`GET /api/v1/search`): the servers the user is in, the
channels they can see in them (a channel hidden from them by its overrides is
left out), and the usernames and display names of people they share a server
with. Matching ignores case and finds the words anywhere in a name; `%`, `_`
and `\` are taken literally. Queries are 2 to 100 characters, and up to 20 of
each kind come back.

The search screen sends the words to the server only after the user asks it
to ("Also find servers, channels and people by name"), so a message search
alone stays on the device. Searching from inside a conversation or channel
does not offer it.

## Not built

Searchable symmetric encryption and server-readable channels are not built;
see ADR 0007.
