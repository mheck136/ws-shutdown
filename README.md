# WS Shutdown

`Server.Shutdown(context.Context)` of the `http` package of the Go standard library does not take hijacked connections
into account. Websocket implementations like [gorilla/websocket](https://github.com/gorilla/websocket) do use connection
hijacking to upgrade HTTP/1 connections to websocket connections.

**From pkg.go.dev in package http – func (\*Server) Shutdown** ([source](https://pkg.go.dev/net/http#Server.Shutdown))

> Shutdown does not attempt to close nor wait for hijacked connections such as WebSockets. The caller of Shutdown should
> separately notify such long-lived connections of shutdown and wait for them to close, if desired. See
> Server.RegisterOnShutdown for a way to register shutdown notification functions.

Left unaddressed, this will lead to dropped active websocket connections when shutting down a server application. While
that might be acceptable in some situations, it's generally considered a best practice to close connections gracefully
and according to protocol. In the case of websockets, that includes sending close messages.

## Shutdowner

This module provides a very simple solution to the problem. It's compatible with the `http.Handler` interface and
provides a middleware function to wrap an `http.Handler` that hijacks connections and a `Shutdown(context.Context)`
function that waits for all handlers wrapped by the middleware to return. There is also a convenience function
`ShutdownWithServer(context.Context, *http.Server) error` that invokes Shutdown on both, the Server and the Shutdowner
in separate Goroutines as this is the most common usage scenario. See the Go Docs and the example below for more details.

### Considerations

This implementation does not monitor the underlying hijacked `net.Conn` connections, but instead monitors that all
http.Handlers have returned. It is possible to keep using a hijacked connection even after the corresponding
http.Handler has returned, but for such scenarios, this package is not suited.

When using this package, it is important to ensure that hijacked connections can safely be considered closed when the
corresponding `http.Handler` returns.

## Example usage
```Go
ctx := context.Background()

// a single instance per application should be enough
shutdowner := shutdown.NewShutdowner()

// handler that hijacks connections, e.g. for websockets
var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // here, the hijacking of the connection would happen, e.g. upgrading to a websocket connection
    w.WriteHeader(http.StatusOK)
})

// wrap the handler with the shutdowner middleware
handler = shutdowner.Middleware(handler)

server := http.Server{
    Addr:    ":8080",
    Handler: handler,
}

// start the server
go func() {
    err := server.ListenAndServe()
    if err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Printf("server reported an error: %v", err)
    }
}()

// wait for the interrupt signal
signalCtx, cancel := signal.NotifyContext(ctx, os.Interrupt)
defer cancel()

// simulate signal
go func() {
    time.Sleep(1 * time.Second)
    _ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}()

<-signalCtx.Done()

// set a timeout for the shutdown
shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()

// shutdown the server gracefully
err := shutdowner.ShutdownWithServer(shutdownCtx, &server)
if err != nil {
    log.Printf("graceful server shutdown failed: %v", err)
    return
}

log.Println("server shutdown gracefully")
```
