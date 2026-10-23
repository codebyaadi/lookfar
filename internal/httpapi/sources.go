package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/codebyaadi/lookfar/internal/source"
	"github.com/codebyaadi/lookfar/internal/store"
)

const (
	maxSourcesPerUser   = 100
	minInterval         = 300
	maxInterval         = 30 * 24 * 3600
	defaultInterval     = 3600
	maxTitleLen         = 200
	checkTimeout        = 40 * time.Second
	checkNowCooldown    = 15 * time.Second
	snapshotListLimit   = 100
	pgUniqueViolation   = "23505"
	checkedJustNowError = "this source was checked a moment ago; try again in a few seconds"
)

type sources struct {
	q   *store.Queries
	svc *source.Service
}

func (h *sources) routes(r chi.Router) {
	r.Get("/sources", h.list)
	r.Post("/sources", h.create)
	r.Route("/sources/{id}", func(r chi.Router) {
		r.Get("/", h.get)
		r.Patch("/", h.update)
		r.Delete("/", h.remove)
		r.Post("/check", h.checkNow)
		r.Get("/snapshots", h.listSnapshots)
		r.Get("/snapshots/{snapshotId}", h.getSnapshot)
	})
}

type sourceView struct {
	ID            int64           `json:"id"`
	Kind          string          `json:"kind"`
	URL           string          `json:"url"`
	Title         string          `json:"title"`
	Config        json.RawMessage `json:"config"`
	IntervalSecs  int32           `json:"intervalSecs"`
	Paused        bool            `json:"paused"`
	NextCheckAt   time.Time       `json:"nextCheckAt"`
	LastCheckedAt *time.Time      `json:"lastCheckedAt"`
	LastChangedAt *time.Time      `json:"lastChangedAt"`
	ErrorCount    int32           `json:"errorCount"`
	LastError     *string         `json:"lastError"`
	CreatedAt     time.Time       `json:"createdAt"`
	SnapshotCount *int32          `json:"snapshotCount,omitempty"`
}

func viewOf(s store.Source) sourceView {
	cfg := json.RawMessage(s.Config)
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	}
	return sourceView{
		ID: s.ID, Kind: s.Kind, URL: s.Url, Title: s.Title, Config: cfg, IntervalSecs: s.IntervalSecs,
		Paused: s.Paused, NextCheckAt: s.NextCheckAt, LastCheckedAt: s.LastCheckedAt, LastChangedAt: s.LastChangedAt,
		ErrorCount: s.ErrorCount, LastError: s.LastError, CreatedAt: s.CreatedAt,
	}
}

type checkView struct {
	OK             bool    `json:"ok"`
	Error          *string `json:"error,omitempty"`
	Baseline       bool    `json:"baseline,omitempty"`
	Changed        bool    `json:"changed,omitempty"`
	NotModified    bool    `json:"notModified,omitempty"`
	BelowThreshold bool    `json:"belowThreshold,omitempty"`
	SnapshotID     *int64  `json:"snapshotId,omitempty"`
}

func (h *sources) list(w http.ResponseWriter, r *http.Request) {
	rows, err := h.q.ListSources(r.Context(), userFrom(r.Context()).ID)
	if err != nil {
		internalError(w, r, "list sources", err)
		return
	}
	out := make([]sourceView, 0, len(rows))
	for _, row := range rows {
		v := viewOf(store.Source{
			ID: row.ID, UserID: row.UserID, Kind: row.Kind, Url: row.Url, Title: row.Title, Config: row.Config,
			IntervalSecs: row.IntervalSecs, NextCheckAt: row.NextCheckAt, Etag: row.Etag, LastModified: row.LastModified,
			LastCheckedAt: row.LastCheckedAt, LastChangedAt: row.LastChangedAt, ErrorCount: row.ErrorCount,
			LastError: row.LastError, Paused: row.Paused, CreatedAt: row.CreatedAt,
		})
		n := row.SnapshotCount
		v.SnapshotCount = &n
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": out})
}

type createRequest struct {
	Kind         string        `json:"kind"`
	URL          string        `json:"url"`
	Title        string        `json:"title"`
	IntervalSecs int32         `json:"intervalSecs"`
	Config       source.Config `json:"config"`
}

func (h *sources) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user := userFrom(r.Context())

	if req.Kind == "" {
		req.Kind = source.KindPage
	}
	if len(req.Title) > maxTitleLen {
		writeError(w, http.StatusBadRequest, "bad_request", "title is too long")
		return
	}
	if req.IntervalSecs == 0 {
		req.IntervalSecs = defaultInterval
	}
	if req.IntervalSecs < minInterval || req.IntervalSecs > maxInterval {
		writeError(w, http.StatusBadRequest, "bad_request", "intervalSecs must be between 300 and 2592000")
		return
	}
	if err := req.Config.Validate(req.Kind); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	canonical, err := source.CanonicalURL(req.Kind, req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	n, err := h.q.CountSources(r.Context(), user.ID)
	if err != nil {
		internalError(w, r, "count sources", err)
		return
	}
	if n >= maxSourcesPerUser {
		writeError(w, http.StatusConflict, "limit_reached", "you can watch at most "+strconv.Itoa(maxSourcesPerUser)+" sources")
		return
	}

	cfg, _ := json.Marshal(req.Config)
	src, err := h.q.CreateSource(r.Context(), store.CreateSourceParams{
		UserID: user.ID, Kind: req.Kind, Url: canonical, Title: req.Title, Config: cfg, IntervalSecs: req.IntervalSecs,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		writeError(w, http.StatusConflict, "duplicate", "you are already watching this URL")
		return
	}
	if err != nil {
		internalError(w, r, "create source", err)
		return
	}

	// Run the first check now so the user sees a baseline (or the problem) at once.
	check := h.runCheck(r.Context(), src.ID)
	fresh, err := h.q.GetSource(r.Context(), store.GetSourceParams{ID: src.ID, UserID: user.ID})
	if err != nil {
		fresh = src
	}
	writeJSON(w, http.StatusCreated, map[string]any{"source": viewOf(fresh), "check": check})
}

// runCheck performs a check and describes it. A check that fails because of
// the target (not our database) is reported in the result, not as an HTTP error.
func (h *sources) runCheck(ctx context.Context, id int64) checkView {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	out, err := h.svc.CheckSource(ctx, id)
	if err != nil {
		msg := err.Error()
		var ce *source.CheckError
		if !errors.As(err, &ce) {
			msg = "the check could not be completed"
		}
		return checkView{OK: false, Error: &msg}
	}
	v := checkView{OK: true, Baseline: out.Baseline, Changed: out.Changed, NotModified: out.NotModified, BelowThreshold: out.BelowThreshold}
	if out.Snapshot != nil {
		v.SnapshotID = &out.Snapshot.ID
	}
	return v
}

func sourceID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusNotFound, "not_found", "source not found")
		return 0, false
	}
	return id, true
}

func (h *sources) load(w http.ResponseWriter, r *http.Request) (store.Source, bool) {
	id, ok := sourceID(w, r)
	if !ok {
		return store.Source{}, false
	}
	src, err := h.q.GetSource(r.Context(), store.GetSourceParams{ID: id, UserID: userFrom(r.Context()).ID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "source not found")
		return store.Source{}, false
	}
	if err != nil {
		internalError(w, r, "get source", err)
		return store.Source{}, false
	}
	return src, true
}

func (h *sources) get(w http.ResponseWriter, r *http.Request) {
	if src, ok := h.load(w, r); ok {
		writeJSON(w, http.StatusOK, map[string]any{"source": viewOf(src)})
	}
}

type updateRequest struct {
	Title        *string        `json:"title"`
	IntervalSecs *int32         `json:"intervalSecs"`
	Paused       *bool          `json:"paused"`
	Config       *source.Config `json:"config"`
}

func (h *sources) update(w http.ResponseWriter, r *http.Request) {
	src, ok := h.load(w, r)
	if !ok {
		return
	}
	var req updateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	p := store.UpdateSourceParams{ID: src.ID, UserID: src.UserID, Title: src.Title, Config: src.Config, IntervalSecs: src.IntervalSecs, Paused: src.Paused}
	if req.Title != nil {
		if len(*req.Title) > maxTitleLen {
			writeError(w, http.StatusBadRequest, "bad_request", "title is too long")
			return
		}
		p.Title = *req.Title
	}
	if req.IntervalSecs != nil {
		if *req.IntervalSecs < minInterval || *req.IntervalSecs > maxInterval {
			writeError(w, http.StatusBadRequest, "bad_request", "intervalSecs must be between 300 and 2592000")
			return
		}
		p.IntervalSecs = *req.IntervalSecs
	}
	if req.Paused != nil {
		p.Paused = *req.Paused
	}
	if req.Config != nil {
		if err := req.Config.Validate(src.Kind); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		p.Config, _ = json.Marshal(req.Config)
	}

	updated, err := h.q.UpdateSource(r.Context(), p)
	if err != nil {
		internalError(w, r, "update source", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"source": viewOf(updated)})
}

func (h *sources) remove(w http.ResponseWriter, r *http.Request) {
	id, ok := sourceID(w, r)
	if !ok {
		return
	}
	n, err := h.q.DeleteSource(r.Context(), store.DeleteSourceParams{ID: id, UserID: userFrom(r.Context()).ID})
	if err != nil {
		internalError(w, r, "delete source", err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "not_found", "source not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *sources) checkNow(w http.ResponseWriter, r *http.Request) {
	src, ok := h.load(w, r)
	if !ok {
		return
	}
	if src.LastCheckedAt != nil && time.Since(*src.LastCheckedAt) < checkNowCooldown {
		writeError(w, http.StatusTooManyRequests, "too_many_requests", checkedJustNowError)
		return
	}
	check := h.runCheck(r.Context(), src.ID)
	fresh, err := h.q.GetSource(r.Context(), store.GetSourceParams{ID: src.ID, UserID: src.UserID})
	if err != nil {
		internalError(w, r, "reload source", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"source": viewOf(fresh), "check": check})
}

type snapshotSummary struct {
	ID           int64     `json:"id"`
	FetchedAt    time.Time `json:"fetchedAt"`
	AddedLines   int32     `json:"addedLines"`
	RemovedLines int32     `json:"removedLines"`
	SizeBytes    int32     `json:"sizeBytes"`
	Baseline     bool      `json:"baseline"`
}

func (h *sources) listSnapshots(w http.ResponseWriter, r *http.Request) {
	src, ok := h.load(w, r)
	if !ok {
		return
	}
	rows, err := h.q.ListSnapshots(r.Context(), store.ListSnapshotsParams{SourceID: src.ID, RowLimit: snapshotListLimit})
	if err != nil {
		internalError(w, r, "list snapshots", err)
		return
	}
	out := make([]snapshotSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, snapshotSummary{ID: row.ID, FetchedAt: row.FetchedAt, AddedLines: row.AddedLines, RemovedLines: row.RemovedLines, SizeBytes: row.SizeBytes, Baseline: row.Baseline})
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": out})
}

func (h *sources) getSnapshot(w http.ResponseWriter, r *http.Request) {
	src, ok := h.load(w, r)
	if !ok {
		return
	}
	snapID, err := strconv.ParseInt(chi.URLParam(r, "snapshotId"), 10, 64)
	if err != nil || snapID < 1 {
		writeError(w, http.StatusNotFound, "not_found", "snapshot not found")
		return
	}
	snap, err := h.q.GetSnapshot(r.Context(), store.GetSnapshotParams{ID: snapID, SourceID: src.ID, UserID: src.UserID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "snapshot not found")
		return
	}
	if err != nil {
		internalError(w, r, "get snapshot", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshot": map[string]any{
		"id": snap.ID, "fetchedAt": snap.FetchedAt, "addedLines": snap.AddedLines, "removedLines": snap.RemovedLines,
		"sizeBytes": snap.SizeBytes, "baseline": snap.Diff == nil, "content": snap.Content, "diff": json.RawMessage(snap.Diff),
	}})
}
