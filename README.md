# Engine.IO client (Go)

[![Go Reference](https://pkg.go.dev/badge/github.com/zishang520/engine.io-client-go.svg)](https://pkg.go.dev/github.com/zishang520/engine.io-client-go)

This is the Go client implementation for [Engine.IO](http://github.com/zishang520/engine.io), the cross-platform bidirectional communication layer for [Socket.IO](http://github.com/zishang520/socket.io).

## Installation

```bash
go get github.com/zishang520/engine.io-client-go
```

## Usage

### Basic Usage

```go
package main

import (
    "log"
    eio "github.com/zishang520/engine.io-client-go/engine"
)

func main() {
    socket := eio.NewSocket("ws://localhost", nil)
    socket.On("open", func(args ...any) {
        utils.SetTimeout(func() {
            e.Send(types.NewStringBufferString("88888"), nil, nil)
        }, 1*time.Second)
        utils.Log().Debug("close %v", args)
    })
}
```

### Connection with Options

```go
package main

import (
    "github.com/zishang520/engine.io-client-go/engine"
    "github.com/zishang520/engine.io-client-go/transports"
    "github.com/zishang520/engine.io/v2/types"
)

func main() {
    opts := engine.DefaultSocketOptions()
    opts.SetPath("/engine.io")
    opts.SetQuery(map[string][]string{
        "token": {"abc123"},
    })
    opts.SetTransports(types.NewSet(transports.WebSocket))

    socket := engine.NewSocket("ws://localhost", opts)

    // ... Handle connection events
}
```

## Features

- Lightweight
- Multiple transport support (WebSocket, Polling, WebTransport)
- Binary data support
- Automatic reconnection
- Custom transport options
- Full Engine.IO protocol compatibility

## API

### Socket

The main client class.

#### Options

- `Transports` (`[]string`): List of transports to try, defaults to `["polling", "websocket", "webtransport"]`
- `Path` (`string`): Connection path, defaults to `/engine.io`
- `Query` (`map[string]string`): Connection query parameters
- `Upgrade` (`bool`): Whether to try upgrading transport, defaults to true
- `RememberUpgrade` (`bool`): Whether to remember successful WebSocket connections, defaults to false
- `RequestTimeout` (`time.Duration`): HTTP request timeout
- `ExtraHeaders` (`http.Header`): Additional HTTP headers
...

#### Events

- `open`: Fired upon successful connection
- `message`: Fired when data is received
- `close`: Fired upon disconnection
- `error`: Fired when an error occurs
- `ping`: Fired when a ping packet is received
- `pong`: Fired when a pong packet is sent
- `upgrade`: Fired upon successful transport upgrade
- `upgradeError`: Fired when transport upgrade fails

#### Methods

- `Send(io.Reader, *packet.Options, func()) engine.SocketWithoutUpgrade`: Send a message
- `Close() engine.SocketWithoutUpgrade`: Close the connection

## Development

To contribute code or run tests, clone the repository:

```bash
git clone https://github.com/zishang520/engine.io-client-go.git
cd engine.io-client-go
go test ./...
```

## License

MIT - Copyright (c) 2024
