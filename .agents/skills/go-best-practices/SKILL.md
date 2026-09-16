---
name: go-best-practices
description: >-
  Comprehensive guide and standards for Go (Golang) development following Effective Go, 
  Uber Go Style Guide, and production desktop/backend architecture. 
  Use when writing, refactoring, or reviewing Go code to ensure idiomatic error handling, 
  thread-safe concurrency, clean package layouts, and robust Windows desktop integrations.
---

# Go (Golang) Production Best Practices Guide

This skill defines the coding standards, design patterns, and engineering practices for building robust, idiomatic, high-performance Go software, specifically covering desktop executables and backend services.

---

## 🧭 1. Core Idioms & Code Style

### Error Handling & Wrapping
- **Always handle errors explicitly:** Never discard errors with `_ = err` unless explicitly documented why.
- **Wrap errors with context:** Use `fmt.Errorf("operation failed: %w", err)` to preserve error chains.
- **Check errors immediately:** Keep the happy path left-aligned.
  ```go
  // GOOD: Guard clause, happy path left-aligned
  result, err := doSomething(ctx)
  if err != nil {
      return nil, fmt.Errorf("doSomething: %w", err)
  }
  return result, nil
  ```
- **Error types:** Use `errors.Is(err, target)` and `errors.As(err, &target)` instead of direct equality checks.

### Context Propagation
- `context.Context` must always be the **first parameter** in functions performing I/O, network requests, database operations, or long-running tasks:
  ```go
  func (c *Client) FetchData(ctx context.Context, id string) (*Data, error)
  ```
- Always respect cancellation and timeouts:
  ```go
  select {
  case <-ctx.Done():
      return ctx.Err()
  case res := <-ch:
      return process(res)
  }
  ```
- Never store a `Context` inside a struct; pass it through the call stack.

---

## ⚡ 2. Concurrency & Goroutines

### Goroutine Lifecycle & Leak Prevention
- **Never start a goroutine without knowing how it will stop.**
- Every long-running goroutine must listen to a `ctx.Done()` or a shutdown channel.
- Always use `sync.WaitGroup` or `errgroup.Group` to wait for spawned goroutines before exiting a parent function:
  ```go
  var wg sync.WaitGroup
  for _, item := range items {
      wg.Add(1)
      go func(it Item) {
          defer wg.Done()
          process(it)
      }(item)
  }
  wg.Wait()
  ```

### Channel Hygiene
- Only the **sender** should close a channel, never the receiver.
- Closing a channel signals completion to multiple receivers.
- Avoid unbuffered channels when sender and receiver operate at bursty rates, but do not use large buffers as queues without backpressure mechanisms.

### Mutexes & State Protection
- Keep locks as close as possible to the critical section.
- Use `defer mu.Unlock()` immediately after acquiring the lock to prevent deadlocks:
  ```go
  s.mu.Lock()
  defer s.mu.Unlock()
  ```
- Use `sync.RWMutex` when reads heavily outnumber writes.

---

## 🏗️ 3. Package Structure & Architecture

Follow standard Go application structure:
```
project/
├── cmd/
│   └── app/               # Main package, flags, startup, dependency wiring only
├── internal/              # Private application code (not importable by other modules)
│   ├── api/               # HTTP handlers, routing, middleware
│   ├── engine/            # Core business logic / worker engines
│   ├── company/           # Domain entity and persistence logic
│   └── secrets/           # Platform security, encryption, DPAPI
└── web/                   # Embedded or static web assets (HTML, CSS, JS)
```

### Dependency Injection & Constructors
- Return concrete structs from constructors (`NewX(...) *X`), accept interfaces in consumers.
- Keep constructors pure: do not perform blocking network calls or start goroutines inside `New(...)`; use an explicit `Start(ctx)` or `Run(ctx)` method.

---

## 🖥️ 4. Windows Desktop & Executable Specifics

### Cero Consola Negra (GUI Mode)
- Compile desktop apps without console window:
  ```powershell
  go build -ldflags="-H windowsgui -s -w" -o AppName.exe ./cmd/server
  ```
- `-s`: Strip symbol table (reduces binary size).
- `-w`: Strip DWARF debugging information (reduces binary size ~30%).
- `-H windowsgui`: Prevents the Windows command prompt (`cmd.exe`) popup on launch.

### Graceful Shutdown
- Listen to OS interrupt signals (`os.Interrupt`, `syscall.SIGTERM`):
  ```go
  ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
  defer stop()
  ```
- Provide clean teardown for HTTP servers (`server.Shutdown(ctx)`), database connections, and background workers.

### Local Data & Paths
- Never assume the working directory is writable (e.g. `Program Files` requires admin rights).
- Always store user data and databases in `%APPDATA%` or user-configured directories:
  ```go
  appData, _ := os.UserConfigDir()
  dataDir := filepath.Join(appData, "AutoSire")
  ```

---

## 🧪 5. Testing & Verification

- Write table-driven tests for business logic and parsers.
- Use subtests with `t.Run(tc.name, func(t *testing.T) { ... })`.
- Run race detection during validation:
  ```powershell
  go test -race ./...
  ```
- Always verify that code formats properly with `gofmt -s -w .` or `go vet ./...`.
