# Unified GoLang Project: Chat, gRPC, and Web Server

## Project Overview

This is a comprehensive Go project that demonstrates three progressive assignments integrated into a single unified codebase. The project showcases fundamental to advanced Go concepts including **concurrency** with a real-time chat application, **Remote Procedure Calls (RPC)** with a gRPC key-value store, and basic web serving.

Initially, you provided requirements for **three** distinct applications, which have been unified into this single project repository.

## AI Prompt to Generate This Project

To generate a similar project using an AI tool, you can use the following modified and precise prompt:

> "Create a unified Go project with three distinct components.
> **1. Concurrent Chat Server:** Build a WebSocket-based chat server (`main.go`, `hub.go`, `client.go`) that allows multiple clients to connect and broadcast messages to each other in real-time. Use goroutines and channels for concurrency management.
> **2. gRPC Key-Value Store:** Implement a gRPC key-value store. Define the service with a `.proto` file (`store.proto`) for `Set`, `Get`, and `Delete` operations. Implement the gRPC server (`store-app/main.go` and `store/store.go`) and a separate command-line client (`client/client.go`) to interact with it.
> **3. Static Web Server:** The main application should also function as a simple web server, capable of serving static HTML files (`about.html`, `list.html`).
>
> Ensure the project is well-structured with appropriate packages (`client`, `store`, `proto`). The project must be initialized as a Go module and include unit tests for the store logic (`store/store_test.go`) and the main application (`main_test.go`). Provide clear instructions on how to generate gRPC code from the proto file, run each server, and use the client."

---

## Assignment Details

*   **Assignment 1: Concurrent Chat Server & Web Server**
    *   **Objective:** Build a real-time chat application that can handle multiple simultaneous client connections using WebSockets.
    *   **Features:** A central hub manages client connections, message broadcasting, and client registration/unregistration. It also serves static HTML files.
    *   **Concepts:** Concurrency (goroutines and channels), WebSockets, HTTP server.

*   **Assignment 2: gRPC Key-Value Store**
    *   **Objective:** Implement a high-performance key-value storage system using gRPC.
    *   **Features:** A server exposes `Set`, `Get`, and `Delete` operations. A separate client application can call these remote procedures.
    *   **Concepts:** Protocol Buffers (Protobuf), gRPC, client-server architecture, RPC.

*   **Assignment 3: gRPC Client**
    *   **Objective:** Create a command-line interface (CLI) client to interact with the gRPC server.
    *   **Features:** The client can send `Set`, `Get`, or `Delete` requests to the gRPC server from the command line.
    *   **Concepts:** Command-line flag parsing, gRPC client implementation.

---

## How to Run and Execute

### 1. Prerequisites
*   Go (version 1.18 or later)
*   Protocol Buffers Compiler (`protoc`)

### 2. Setup
```bash
# Clone the repository (if you haven't already)
# git clone <repository-url>
# cd unified-go-prog

# Install dependencies
go mod tidy

# Generate gRPC code from the .proto file
# This command uses the protoc compiler to generate the necessary Go files for gRPC
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/store.proto
```

### 3. Running the Applications

You need to run the two servers (the Chat/Web server and the gRPC server) in separate terminal windows.

**Terminal 1: Start the Chat and Web Server**
```bash
# This command starts the main server for chat and static pages on port 8080
go run main.go
```
*   **Web Pages:** Open your browser and navigate to `http://localhost:8080/about.html` or `http://localhost:8080/list.html`.
*   **Chat:** Use a WebSocket client to connect to `ws://localhost:8080/ws` to join the chat.

**Terminal 2: Start the gRPC Key-Value Store Server**
```bash
# This command starts the gRPC server on port 50051
go run ./store-app/main.go
```

**Terminal 3: Use the gRPC Client**
The client can perform `set`, `get`, or `delete` operations.

```bash
# Set a key-value pair
go run ./client/client.go -op=set -key="name" -val="Gemini"

# Get the value for a key
go run ./client/client.go -op=get -key="name"

# Delete a key
go run ./client/client.go -op=delete -key="name"
```

### 4. Running Tests
To ensure all components are working as expected, run the project's tests.
```bash
# This command will execute all test files in the project
go test ./... -v
```
