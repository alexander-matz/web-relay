# web-relay

web-relay is an smtp relay accessible from websites. The purpose is to allow
client-side javascript applications to send e-mails without polyfills etc.
Security obviously is a big issue with stuff like this (think spam), so I'm trying
to implement as many filters as makes sense in order to close down the relay for
unauthorized access.

## Usage (server)

1. Download source
2. Get dependencies (kingpin) with `go get` in source directory
3. Build with `go build`
4. Start resulting binary

```
usage: web-relay --bindaddr=BINDADDR --server=SERVER [<flags>]

Flags:
  --help               Show context-sensitive help (also try --help-long and --help-man).
  --bindaddr=BINDADDR  Address this server should bind to.
  --server=SERVER      Target relay mail server.
  --secret=""          Secret required to access server.
```

## Usage (client)

The client has access to two API endpoints

- `/plain` accepts POST requests for simple and plain (non-templated) e-mails
- `/template` accepts POST requests for templated e-mails

Shared POST fields are:

- `secret`: simple (and bad) authentication
- `from`: required exactly once, sender e-mail address
- `to`: required once or more, receiver e-mail address(es)
- `subject`: required exactly once, e-mail subject
- `message`: required exactly once, e-mail body

The templated endpoint also expects the parameter `values` that is required once or more
and holds a row of values for each instance of the template. Each row follows the format
`"name"="value"[,more...]`.

## Features

(✓ indicates "implemented", ✗ indicates "missing")

- ✓ plain + templated mails (templates are expanded on the server to save bandwidth)
- ✓ shared secret authorization
- ✗ filter by sender
- ✗ other nice filters

## License

This project is licensed under MIT license which allows use of this application for
non-commercial as well as commercial use. See LICENSE for more information.
