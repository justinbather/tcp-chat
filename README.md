# tcp-chat

A group chat server and terminal client over raw TCP, in Go. No HTTP, no WebSockets — a
`net.Listener`, a goroutine per connection, and `encoding/gob` on the wire.

Built to understand what a chat server is actually doing underneath a framework.

## Run it

Two terminals, or more.

```bash
# server, listens on :8000
go run ./server

# client, one per participant
go run ./client
```

Type a line and press enter to broadcast it to everyone connected. `/help` lists commands.
`Ctrl+C` disconnects cleanly.

## How it works

**Server.** `run` accepts connections in a loop and hands each one to `handleConn` in its own
goroutine. Every client gets two more goroutines — `readInput` decoding gob frames off the
socket, and `writeOutput` encoding them back — communicating over a pair of buffered-free
channels (`Incoming`, `Outgoing`). `handleConn` ranges over `Incoming` and fans each message
out to every client's `Outgoing` via `Lobby.Broadcast`. A client that disconnects hits `io.EOF`,
which closes its channels and removes it from the lobby.

**Client.** Three goroutines and a `select`: one scanning stdin, one decoding incoming frames,
and the main loop multiplexing between new input and `SIGINT`. Server-side EOF is turned into
a synthetic interrupt so the client shuts down when the lobby goes away.

**Wire format.** `gob`-encoded `ChatMsg{Content, Sender, Id}` structs, read into a fixed 1KB buffer.

## Known limitations

This was a learning exercise and it has the bugs to prove it. Listed rather than hidden:

- **Data races.** `Lobby.Clients` is appended to and spliced from multiple goroutines with no
  mutex, and the `CURR_ID` counter is incremented unsynchronised. Run it under `-race` and it
  will tell you so. The fix is either a mutex on the lobby or funnelling all mutations through
  a single owning goroutine.
- **No message framing.** Each read grabs up to 1024 bytes and hands the buffer to a gob decoder,
  which assumes one complete message per read. A large message, or two messages arriving in one
  TCP segment, will not decode correctly. Real fix: length-prefix each frame.
- **Lobbies are advertised but not implemented.** `/help` lists `/create` and `/join`; the command
  switch only handles `help` and a no-op `new`. Everyone shares one implicit lobby.
- **Four clients maximum**, and names are assigned from a hardcoded four-element list, so the cap
  is really "how many names are in the array." The server stops accepting — and returns from its
  accept loop entirely — once it's full.
- **Startup log says `:8001`** while the listener binds `:8000`.
- No auth, no TLS, no persistence, no reconnection.

## What I'd do differently

Single owner goroutine for lobby state with clients communicating by channel, length-prefixed
framing instead of fixed-size reads, and a `context.Context` threaded through shutdown instead
of relaying a signal between goroutines.
