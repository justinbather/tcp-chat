# TCP Chat server

### Goals:
- Build a fully functional tcp chat server through tests with a go sdk for client integration
- Ditching the tui for now, just work on supporting a generic way to:
    - accept connections
    - complete an auth handshake
    - handle lobbys
    - message sending/persisting
    - users

### Design Philosophy:
The nature of this project is to build a really good library essentially (and learn the infra side of things with the concurrency).
> 1. Everything written must follow the *design* -> *test* -> *implement* approach. First design the api, then implement it in a test-driven fashion.
> 2. Think from the SDK-side of the equation. How would you ideally want to interact with the service?

By the time the service works, there should be a client SDK ready to go with it. Building a tui client etc should be easy since we'll have a nice client to do all the dirty work

### Must Haves:
- The client shouldn't have to worry about order of steps to connect, or have to think how to integrate with the service. eg
```go
conn, err := exampleService.Connect(user)
if err != nil {
    // handle err
    // maybe auth didnt work? 
    // maybe url is wrong?
    // maybe permissions aren't correct?
}

// conn could store all our connected channels data, user data etc

// use connection as you wish

// joining a new channel
// channel could be a struct that includes a method to recieve chats
channel, err := conn.JoinChannel("channel")

// sending message into channel
err := channel.SendMessage(message)

// send message to all channels
err := conn.Broadcast(message)
```
