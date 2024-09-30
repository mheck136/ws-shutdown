package shutdown_test

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	shutdown "github.com/mheck136/ws-shutdown"
)

func ExampleShutdowner() {
	ctx := context.Background()

	// a single instance per application should be enough
	var shutdowner shutdown.Shutdowner

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
	//Output:
}

func TestShutdowner(t *testing.T) {
	t.Parallel()
	newSleepingHandler := func(d time.Duration) (http.Handler, chan struct{}) {
		started := make(chan struct{})
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			time.Sleep(d)
		}), started
	}

	tt := []struct {
		name                   string
		durations              []time.Duration
		shutdownAfter          time.Duration
		shutdownTimeout        time.Duration
		expectDeadlineExceeded bool
	}{
		{
			name:                   "shutdown before handler finishes - deadline exceeded",
			durations:              []time.Duration{10 * time.Millisecond},
			shutdownAfter:          0,
			shutdownTimeout:        5 * time.Millisecond,
			expectDeadlineExceeded: true,
		},
		{
			name:                   "shutdown before handler finishes - deadline not exceeded",
			durations:              []time.Duration{5 * time.Millisecond},
			shutdownAfter:          0,
			shutdownTimeout:        10 * time.Millisecond,
			expectDeadlineExceeded: false,
		},
		{
			name:                   "shutdown before multiple handlers finish - deadline exceeded",
			durations:              []time.Duration{10 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond},
			shutdownAfter:          0,
			shutdownTimeout:        5 * time.Millisecond,
			expectDeadlineExceeded: true,
		},
		{
			name:                   "shutdown before multiple handlers finish - deadline not exceeded",
			durations:              []time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond},
			shutdownAfter:          0,
			shutdownTimeout:        10 * time.Millisecond,
			expectDeadlineExceeded: false,
		},
		{
			name:                   "shutdown after handler finishes",
			durations:              []time.Duration{5 * time.Millisecond},
			shutdownAfter:          10 * time.Millisecond,
			shutdownTimeout:        5 * time.Millisecond,
			expectDeadlineExceeded: false,
		},
		{
			name:                   "shutdown after multiple handlers finish",
			durations:              []time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond},
			shutdownAfter:          10 * time.Millisecond,
			shutdownTimeout:        5 * time.Millisecond,
			expectDeadlineExceeded: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var shutdowner shutdown.Shutdowner

			for _, d := range tc.durations {
				handler, started := newSleepingHandler(d)
				go shutdowner.Middleware(handler).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
				<-started // wait for handler to start
			}

			time.Sleep(tc.shutdownAfter)

			deadlineCtx, cancel := context.WithTimeout(context.Background(), tc.shutdownTimeout)
			defer cancel()
			err := shutdowner.Shutdown(deadlineCtx)
			switch {
			case tc.expectDeadlineExceeded && !errors.Is(err, context.DeadlineExceeded):
				t.Errorf("expected %T, but got %T", context.DeadlineExceeded, err)
			case !tc.expectDeadlineExceeded && err != nil:
				t.Errorf("no error expected but got %v", err)
			}
		})
	}
}
