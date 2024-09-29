package shutdown_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	shutdown "github.com/mheck136/ws-shutdown"
)

func BenchmarkNewGracefulShutdown(b *testing.B) {
	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	w := httptest.NewRecorder()
	var counter int

	shutdowner := shutdown.NewShutdowner()

	handler := shutdowner.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { counter++ }))

	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(w, req)
	}

	err := shutdowner.Shutdown(context.Background())
	if err != nil {
		b.Fatalf("unexpected error: %v", err)
	}
	if counter != b.N {
		b.Fatalf("expected %d, got %d", b.N, counter)
	}
}
