# Security Policy

## Supported versions

Only the latest tagged release is supported. Please upgrade before reporting an issue that may
already be fixed.

## Reporting a vulnerability

Please do **not** open a public GitHub issue for security vulnerabilities.

- **Preferred**: use GitHub's private vulnerability reporting for this repository
  (the "Security" tab → "Report a vulnerability").
- **Alternative**: email olegamysk@gmail.com with details and, if possible, a proof of concept.

You should get an initial response within a few days.

## Scope notes

`Download` makes an outbound HTTP request to a caller-supplied URL and writes a
bzip2-decompressed stream to a caller-supplied destination path — a caller should not pass
an untrusted `dest`, and a caller decompressing an untrusted/hostile `.db.bz2` source should
consider bzip2 decompression-bomb risk (an attacker-controlled source could claim a small
compressed size but expand to something enormous); this package streams straight to disk
without a size cap, since a legitimate planet-scale WOF distribution is itself gigabytes.
`Open`/`Placetype` read a local SQLite file via `modernc.org/sqlite` (a cgo-free driver) and
issue a fixed, parameterized query — no user input is interpolated into SQL. Holds no
credentials/secrets.
