# NetBridge

## Overview

`netbridge` is an open-source package that provides a two-way tunnel for HTTP requests, acting as a proxy over WebSocket. This allows for seamless communication between HTTP clients and servers through a WebSocket connection.

## Features

- **HTTP to WebSocket Proxy**: Translates HTTP requests into WebSocket messages and vice versa.
- **Configurable**: Easily configurable to suit different environments and use cases.
- **Concurrent Handling**: Supports concurrent message handling to ensure efficient communication.
- **Error Handling**: Robust error handling to manage connection issues and message parsing errors.

## Installation

To install `netbridge`, use `go get`:

```sh
go get github.com/niradler/go-netbridge
```

## Usage

### Client

```go
package main

import (
    "log"

    "github.com/niradler/netbridge/config"
    "github.com/niradler/netbridge/shared"
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

    "github.com/niradler/netbridge/config"
    "github.com/niradler/netbridge/shared"
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


## Roadmap

Here are some of the planned features and improvements for `netbridge`:

- **Support for Large Payloads**: Implement chunking and compression to efficiently handle large payloads.
- **Protocol Support**: Extend support to additional protocols such as SSH, HTTP/2, and gRPC.
- **Monitoring and Logging**: Integrate monitoring and logging capabilities to track performance and diagnose issues.
- **Automated Testing**: Develop a comprehensive suite of automated tests to ensure code quality and reliability.
- **Infrastructure as Code**: Provide Terraform scripts and other IaC tools to deploy `netbridge` to various cloud providers
- **Helm Charts**: Develop Helm charts for Kubernetes deployments.
- **CI/CD Integration**: Integrate with CI/CD pipelines for automated deployment and testing.
- **Advanced Configuration Options**: Provide more advanced configuration options for fine-tuning the behavior.
- **Improved Documentation**: Expand the documentation with more examples and detailed explanations.
- **Community Contributions**: Encourage and integrate contributions from the community to add new features and fix bugs.
- **Performance Optimization**: Optimize the performance for handling a large number of concurrent connections.
- **Load Balancing**: Add support for load balancing to distribute traffic across multiple servers.
- **Multi Clients**: Add support for multiple clients.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request with your changes.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Acknowledgements

Special thanks to all contributors and the open-source community for their support.
