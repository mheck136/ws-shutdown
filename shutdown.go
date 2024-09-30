package shutdown

import (
	"context"
	"errors"
	"net/http"
	"sync"
)

// Shutdowner helps with gracefully shutting down http.Handler's that are not taken into account by http/Server.Shutdown
// because the connection has been hijacked. Please be aware that Shutdowner does not monitor the underlying net.Conn
// connection, but only monitors that all http.Handler's wrapped with Middleware have returned. That means that if a
// hijacked connection continues to be used after the http.Handler has returned, Shutdowner will consider that
// connection as inactive and won't prevent the application shutdown from proceeding.
type Shutdowner struct {
	wg sync.WaitGroup
}

// Middleware wraps the invocation of the given handler so that Shutdown can be used to ensure that all handlers have
// returned.
func (g *Shutdowner) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.wg.Add(1)
		defer g.wg.Done()
		next.ServeHTTP(w, r)
	})
}

// Shutdown waits for all active handlers to finish. If the context is cancelled before all handlers finish, the
// function returns the context error. If all handlers finish before the context is cancelled, the function returns nil.
func (g *Shutdowner) Shutdown(ctx context.Context) error {
	d := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(d)
	}()
	select {
	case <-d:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ShutdownWithServer shuts down the server and the shutdowner concurrently, waiting wor both respective Shutdown methods
// to return and returning any errors that occurred with errors.Join.
func (g *Shutdowner) ShutdownWithServer(ctx context.Context, server *http.Server) error {
	var serverErr, shutdownerErr error

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		shutdownerErr = g.Shutdown(ctx)
	}()

	serverErr = server.Shutdown(ctx)
	wg.Wait()

	return errors.Join(serverErr, shutdownerErr)
}
