//go:build integration

package server_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/codebyaadi/lookfar/internal/server"
)

func TestServeHealthAndGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ctr, err := tcpostgres.Run(ctx, "postgres:18",
		tcpostgres.WithDatabase("lookfar"), tcpostgres.WithUsername("lookfar"), tcpostgres.WithPassword("lookfar"),
		tcpostgres.BasicWaitStrategies(),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(ctr) })
	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	runCtx, stop := context.WithCancel(ctx)
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- server.Run(runCtx, server.Options{
			Addr: "127.0.0.1:0", DatabaseURL: url, Version: "test",
			Logger: slog.Default(), Ready: func(a string) { ready <- a },
		})
	}()

	var addr string
	select {
	case addr = <-ready:
	case err := <-done:
		t.Fatalf("server exited early: %v", err)
	case <-time.After(time.Minute):
		t.Fatal("server did not become ready")
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/api/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"status":"ok"`) || !strings.Contains(string(body), `"river":"running"`) {
		t.Fatalf("health: %d %s", resp.StatusCode, body)
	}

	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(time.Minute):
		t.Fatal("server did not shut down")
	}
}
