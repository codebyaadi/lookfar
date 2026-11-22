package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/riverqueue/river/rivertype"
)

type fakeLog struct {
	mu       sync.Mutex
	started  []string
	status   string
	stats    string
	errMsg   string
	finished int
	startErr error
}

func (f *fakeLog) StartRun(_ context.Context, job string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.startErr != nil {
		return 0, f.startErr
	}
	f.started = append(f.started, job)
	return 7, nil
}

func (f *fakeLog) FinishRun(_ context.Context, id int64, status string, stats json.RawMessage, errMsg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id != 7 {
		return errors.New("unexpected run id")
	}
	f.finished++
	f.status, f.stats, f.errMsg = status, string(stats), errMsg
	return nil
}

func work(t *testing.T, log RunLog, inner func(context.Context) error) error {
	t.Helper()
	mw := RunLogMiddleware(log, func(string, ...any) {})
	return mw.Work(t.Context(), &rivertype.JobRow{Kind: "demo"}, inner)
}

func TestRunLogSuccessWithStats(t *testing.T) {
	f := &fakeLog{}
	err := work(t, f, func(ctx context.Context) error {
		RecordStats(ctx, map[string]any{"items": 3})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.status != StatusSucceeded || f.errMsg != "" || f.stats != `{"items":3}` || f.started[0] != "demo" {
		t.Errorf("unexpected log: %+v", f)
	}
}

func TestRunLogFailureKeepsError(t *testing.T) {
	f := &fakeLog{}
	boom := errors.New("boom")
	if err := work(t, f, func(context.Context) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("error not propagated: %v", err)
	}
	if f.status != StatusFailed || f.errMsg != "boom" || f.stats != "{}" {
		t.Errorf("unexpected log: %+v", f)
	}
}

func TestRunLogPanicIsRecordedAndRaised(t *testing.T) {
	f := &fakeLog{}
	defer func() {
		if r := recover(); r == nil {
			t.Error("panic was swallowed")
		}
		if f.status != StatusFailed || !strings.Contains(f.errMsg, "panic: kaboom") {
			t.Errorf("unexpected log: %+v", f)
		}
	}()
	_ = work(t, f, func(context.Context) error { panic("kaboom") })
}

func TestRunLogStartFailureDoesNotFailJob(t *testing.T) {
	f := &fakeLog{startErr: errors.New("db down")}
	if err := work(t, f, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("job should still succeed: %v", err)
	}
	if f.finished != 0 {
		t.Error("finish must be skipped when start failed")
	}
}

func TestRecordStatsOutsideJobIsNoop(t *testing.T) {
	RecordStats(t.Context(), map[string]any{"x": 1}) // must not panic
}

func TestRegistry(t *testing.T) {
	if _, ok := Registry()["heartbeat"]; !ok {
		t.Fatal("heartbeat missing")
	}
	if _, err := heartbeatJob.Args([]string{"extra"}); err == nil {
		t.Error("heartbeat should reject arguments")
	}
}
