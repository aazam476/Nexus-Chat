# Bundled breached-password list

`common-passwords.txt.gz` holds the passwords of 10 or more characters from
SecLists' `Passwords/Common-Credentials/Pwdb_top-1000000.txt` (the most common
passwords in public breach dumps), lower-cased, de-duplicated and sorted.
Shorter passwords are left out because the server already rejects them.

Source: https://github.com/danielmiessler/SecLists (MIT License, see
`LICENSE.SecLists`).

To rebuild it, download that file and keep lines of 10 or more characters,
lower-cased, sorted by byte value, one per line, gzip-compressed.
