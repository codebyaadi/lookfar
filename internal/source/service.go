package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/codebyaadi/lookfar/internal/store"
)

// JobKind is the job-queue kind that checks one source.
const JobKind = "check_source"

// Outcome describes what one check did.
type Outcome struct {
	// Baseline is true for the first snapshot of a source.
	Baseline bool `json:"baseline"`
	// Changed is true when a new snapshot was stored.
	Changed bool `json:"changed"`
	// NotModified is true when the server answered 304.
	NotModified bool `json:"notModified"`
	// BelowThreshold is true when content changed by less than minChangeChars.
	BelowThreshold bool            `json:"belowThreshold"`
	Snapshot       *store.Snapshot `json:"-"`
}

// CheckError is a failure fetching or normalising a source. It has already
// been recorded on the source (with backoff), so callers should not retry.
type CheckError struct{ Err error }

func (e *CheckError) Error() string { return e.Err.Error() }
func (e *CheckError) Unwrap() error { return e.Err }

// DB is what the service needs from the database: queries plus transactions.
type DB interface {
	store.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Service runs checks and stores their results.
type Service struct {
	db      DB
	q       *store.Queries
	checker *Checker
}

func NewService(db DB, c *Checker) *Service {
	return &Service{db: db, q: store.New(db), checker: c}
}

// CheckSource fetches a source, and stores a snapshot when its content changed.
func (s *Service) CheckSource(ctx context.Context, sourceID int64) (Outcome, error) {
	src, err := s.q.GetSourceForCheck(ctx, sourceID)
	if err != nil {
		return Outcome{}, err
	}
	cfg, err := ParseConfig(src.Config)
	if err != nil {
		return Outcome{}, s.fail(ctx, src, err)
	}

	res, err := s.checker.Check(ctx, src.Kind, src.Url, cfg, deref(src.Etag), deref(src.LastModified))
	if err != nil {
		return Outcome{}, s.fail(ctx, src, err)
	}

	checked := store.MarkSourceCheckedParams{ID: src.ID, Title: res.Title, Etag: strPtr(res.ETag), LastModified: strPtr(res.LastModified)}
	if res.NotModified {
		return Outcome{NotModified: true}, s.q.MarkSourceChecked(ctx, checked)
	}

	return s.persist(ctx, src, cfg, res, checked)
}

// persist stores a snapshot when the content changed. It runs in a transaction
// holding a per-source advisory lock, so two checks of one source (the inline
// first check and the scheduler, or two manual checks) cannot both decide the
// content is new and store duplicate snapshots.
func (s *Service) persist(ctx context.Context, src store.Source, cfg Config, res *Result, checked store.MarkSourceCheckedParams) (Outcome, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Outcome{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", src.ID); err != nil {
		return Outcome{}, err
	}
	q := s.q.WithTx(tx)

	finish := func(o Outcome) (Outcome, error) {
		if err := q.MarkSourceChecked(ctx, checked); err != nil {
			return Outcome{}, err
		}
		return o, tx.Commit(ctx)
	}

	prev, err := q.LatestSnapshot(ctx, src.ID)
	hasPrev := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Outcome{}, err
	}

	hash := hashContent(res.Content)
	if hasPrev && prev.ContentHash == hash {
		return finish(Outcome{})
	}

	in := store.InsertSnapshotParams{
		SourceID:    src.ID,
		ContentHash: hash,
		Content:     res.Content,
		SizeBytes:   int32(len(res.Content)),
	}
	if hasPrev {
		d := LineDiff(prev.Content, res.Content)
		if cfg.MinChangeChars > 0 && d.ChangedChars() < cfg.MinChangeChars {
			return finish(Outcome{BelowThreshold: true})
		}
		raw, err := json.Marshal(d)
		if err != nil {
			return Outcome{}, err
		}
		in.Diff, in.AddedLines, in.RemovedLines = raw, int32(d.Added), int32(d.Removed)
	}

	snap, err := q.InsertSnapshot(ctx, in)
	if err != nil {
		return Outcome{}, err
	}
	if err := q.MarkSourceChanged(ctx, src.ID); err != nil {
		return Outcome{}, err
	}
	return finish(Outcome{Baseline: !hasPrev, Changed: true, Snapshot: &snap})
}

// fail records a failed check and wraps the error so callers can tell it from
// a database error.
func (s *Service) fail(ctx context.Context, src store.Source, cause error) error {
	msg := truncate(cause.Error(), 500)
	if err := s.q.MarkSourceFailed(ctx, store.MarkSourceFailedParams{ID: src.ID, LastError: &msg}); err != nil {
		return fmt.Errorf("record failure of source %d: %w", src.ID, err)
	}
	return &CheckError{Err: cause}
}

// ScheduleDue queues a check job for every source whose time has come and
// returns how many it looked at. The dedupe key includes next_check_at, so a
// source that is still queued is not queued twice.
func (s *Service) ScheduleDue(ctx context.Context, batch int) (int, error) {
	due, err := s.q.ListDueSources(ctx, int32(batch))
	if err != nil {
		return 0, err
	}
	for _, d := range due {
		key := "check:" + strconv.FormatInt(d.ID, 10) + ":" + strconv.FormatInt(d.NextCheckAt.UnixMilli(), 10)
		payload, _ := json.Marshal(map[string]int64{"sourceId": d.ID})
		if err := s.q.EnqueueJob(ctx, store.EnqueueJobParams{Kind: JobKind, Payload: payload, DedupeKey: &key}); err != nil {
			return 0, err
		}
	}
	return len(due), nil
}

// Handle is the job handler for JobKind.
func (s *Service) Handle(ctx context.Context, job store.Job) error {
	var p struct {
		SourceID int64 `json:"sourceId"`
	}
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return fmt.Errorf("bad payload: %w", err)
	}
	out, err := s.CheckSource(ctx, p.SourceID)
	var ce *CheckError
	switch {
	case errors.As(err, &ce):
		// Recorded on the source with backoff; retrying the job would double up.
		slog.Warn("source check failed", "source", p.SourceID, "err", ce)
		return nil
	case errors.Is(err, pgx.ErrNoRows):
		return nil // source was deleted
	case err != nil:
		return err
	}
	if out.Changed && !out.Baseline {
		slog.Info("source changed", "source", p.SourceID, "snapshot", out.Snapshot.ID,
			"added", out.Snapshot.AddedLines, "removed", out.Snapshot.RemovedLines)
	}
	return nil
}

func hashContent(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
