package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/codebyaadi/lookfar/internal/store"
)

type inbox struct {
	q *store.Queries
}

func (h *inbox) routes(r chi.Router) {
	r.Get("/inbox", h.list)
}

type inboxItem struct {
	ID         int64      `json:"id"`
	Origin     string     `json:"origin"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	BodyMD     string     `json:"bodyMd"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	ResolvedAt *time.Time `json:"resolvedAt"`
}

func (h *inbox) list(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			writeError(w, http.StatusBadRequest, "bad_request", "limit must be between 1 and 200")
			return
		}
		limit = n
	}
	var status *string
	if v := r.URL.Query().Get("status"); v != "" {
		status = &v
	}

	rows, err := h.q.ListInbox(r.Context(), store.ListInboxParams{
		UserID:   userFrom(r.Context()).ID,
		Status:   status,
		RowLimit: int32(limit),
	})
	if err != nil {
		internalError(w, r, "list inbox", err)
		return
	}

	items := make([]inboxItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, inboxItem{
			ID: row.ID, Origin: row.Origin, Kind: row.Kind, Title: row.Title, BodyMD: row.BodyMd,
			Status: row.Status, CreatedAt: row.CreatedAt, ResolvedAt: row.ResolvedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
