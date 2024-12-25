# NetBridge

## Overview

`go-netbridge` is an open-source package that provides a two-way tunnel for HTTP requests, acting as a proxy over WebSocket. This allows for seamless communication between HTTP clients and servers through a WebSocket connection.

## Features

- **HTTP to WebSocket Proxy**: Translates HTTP requests into WebSocket messages and vice versa.
- **Configurable**: Easily configurable to suit different environments and use cases.
- **Concurrent Handling**: Supports concurrent message handling to ensure efficient communication.
- **Error Handling**: Robust error handling to manage connection issues and message parsing errors.

## Installation

To install `go-netbridge`, use `go get`:

```sh
go get github.com/niradler/go-netbridge
```

## Usage

### Client

```go
package main

import (
    "log"

    "github.com/niradler/go-netbridge/config"
    "github.com/niradler/go-netbridge/shared"
)

func main() {
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatal(err)
    }
    wss, err := shared.NewWebSocketConnection(cfg)
    if err != nil {
        log.Fatalf("Error creating WebSocket server: %v", err)
    }
    defer wss.Close()

    httpServer := shared.NewHTTPServer(cfg, wss)
    log.Fatal(httpServer.Start(":8081"))
}
```

### Server

```go
package main

import (
    "log"

    "github.com/niradler/go-netbridge/config"
    "github.com/niradler/go-netbridge/shared"
)

func main() {
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatal(err)
    }

    httpServer := shared.NewHTTPServer(cfg, nil)

    shared.NewWebSocketServer(httpServer)

    log.Fatal(httpServer.Start(":8080"))
}
```

## Configuration

The configuration is managed through a configuration file. Ensure that the configuration file is correctly set up with the necessary parameters such as `SERVER_URL`, `X_Forwarded_Proto`, and `X_Forwarded_Host`.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request with your changes.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Acknowledgements

Special thanks to all contributors and the open-source community for their support.
