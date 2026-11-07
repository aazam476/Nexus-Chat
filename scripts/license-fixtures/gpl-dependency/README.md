# License check fixture

CI runs `scripts/check-licenses.sh js` against this directory and expects it
to **fail**, which proves the license check actually rejects GPL code. The
`node_modules` folder here is checked in on purpose; it holds only a
`package.json` declaring a GPL license.
