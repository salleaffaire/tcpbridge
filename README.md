# A bridge from TCP to HTTP

## Build

```bash
go build -o tcp-bridge main.go
```

## Run

Let's suppose that we have the following use case

![alt text](TCPB.png)

A calling nc (netcat) client wants to connect to a listening nc client. The connection is over TCP and is initiated by the left side= of the diagram.

Start a Listener bridge on the right side.

```bash
./tcp-bridge --tcpp=8080 --httppout=9090 --httppin=9091 --mode=listener
```

Start a Caller bridge on the lefts side.

```bash
./tcp-bridge --tcpp=8081 --httppout=9091 --httppin=9090 --mode=caller
```

When the calling nc client initiates a connection, it will open it with the listener bridge.

On the first message sent, from the left to right, the listener bridge will send a POST to the caller bridge HTTP server.

The caller bridge will initiate a connection with the listening nc client and send the message.

Beyond that point, bidirectional traffic should flow from,
