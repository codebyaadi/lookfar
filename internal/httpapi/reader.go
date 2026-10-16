package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/codebyaadi/feedoku/internal/ingest"
	"github.com/codebyaadi/feedoku/internal/store"
)

const (
	maxJSONBody   = 1 << 20
	maxOPMLBody   = 5 << 20
	maxOPMLFeeds  = 200
	defaultPage   = 30
	maxPage       = 100
	opmlWorkers   = 5
	addFeedBudget = 45 * time.Second
)

type reader struct {
	q      *store.Queries
	ingest *ingest.Service
}

func (rd *reader) routes(r chi.Router) {
	r.Get("/feeds", rd.listFeeds)
	r.Post("/feeds", rd.addFeed)
	r.Post("/feeds/opml", rd.importOPML)
	r.Patch("/feeds/{id}", rd.updateFeed)
	r.Delete("/feeds/{id}", rd.deleteFeed)

	r.Get("/topics", rd.listTopics)
	r.Get("/articles", rd.listArticles)
	r.Post("/articles/read-all", rd.markAllRead)
	r.Get("/articles/{id}", rd.getArticle)
	r.Post("/articles/{id}/state", rd.setState)
}

// --- feeds ---------------------------------------------------------------

type feedDTO struct {
	ID          int64   `json:"id"`
	URL         string  `json:"url"`
	SiteURL     *string `json:"siteUrl"`
	Title       string  `json:"title"`
	Favicon     *string `json:"favicon"`
	Folder      *string `json:"folder"`
	UnreadCount int64   `json:"unreadCount"`
	Error       *string `json:"error"`
}

func (rd *reader) listFeeds(w http.ResponseWriter, r *http.Request) {
	rows, err := rd.q.ListSubscriptions(r.Context(), userFrom(r.Context()).ID)
	if err != nil {
		serverError(w, r, "list feeds", err)
		return
	}
	out := make([]feedDTO, 0, len(rows))
	for _, f := range rows {
		dto := feedDTO{ID: f.ID, URL: f.Url, SiteURL: f.SiteUrl, Title: f.Title, Favicon: f.Favicon, Folder: f.Folder, UnreadCount: f.UnreadCount}
		if f.ErrorCount >= 3 {
			dto.Error = f.LastError
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, map[string]any{"feeds": out})
}

func (rd *reader) addFeed(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL    string  `json:"url"`
		Folder *string `json:"folder"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.URL) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "url is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), addFeedBudget)
	defer cancel()
	f, err := rd.ingest.AddFeed(ctx, userFrom(r.Context()).ID, in.URL, cleanFolder(in.Folder))
	if err != nil {
		slog.InfoContext(r.Context(), "add feed failed", "url", in.URL, "err", err)
		writeError(w, http.StatusUnprocessableEntity, "feed_not_found", "Couldn't find a feed at that address.")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"feed": feedDTO{ID: f.ID, URL: f.Url, SiteURL: f.SiteUrl, Title: f.Title, Favicon: f.Favicon, Folder: cleanFolder(in.Folder)}})
}

func (rd *reader) updateFeed(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in struct {
		Folder *string `json:"folder"`
		Title  *string `json:"title"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	n, err := rd.q.UpdateSubscription(r.Context(), store.UpdateSubscriptionParams{
		UserID: userFrom(r.Context()).ID, FeedID: id, Folder: cleanFolder(in.Folder), CustomTitle: cleanFolder(in.Title),
	})
	if err != nil {
		serverError(w, r, "update feed", err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "not_found", "feed not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (rd *reader) deleteFeed(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	n, err := rd.q.Unsubscribe(r.Context(), store.UnsubscribeParams{UserID: userFrom(r.Context()).ID, FeedID: id})
	if err != nil {
		serverError(w, r, "unsubscribe", err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "not_found", "feed not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- OPML ----------------------------------------------------------------

type opmlOutline struct {
	Text     string        `xml:"text,attr"`
	Title    string        `xml:"title,attr"`
	XMLURL   string        `xml:"xmlUrl,attr"`
	Children []opmlOutline `xml:"outline"`
}

type opmlDoc struct {
	Body struct {
		Outlines []opmlOutline `xml:"outline"`
	} `xml:"body"`
}

type opmlEntry struct{ url, folder string }

func flattenOPML(outlines []opmlOutline, folder string, out *[]opmlEntry) {
	for _, o := range outlines {
		if o.XMLURL != "" {
			*out = append(*out, opmlEntry{o.XMLURL, folder})
			continue
		}
		name := o.Title
		if name == "" {
			name = o.Text
		}
		if folder == "" {
			flattenOPML(o.Children, name, out)
		} else {
			flattenOPML(o.Children, folder, out)
		}
	}
}

func (rd *reader) importOPML(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxOPMLBody))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "OPML file is too large")
		return
	}
	var doc opmlDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_opml", "That doesn't look like an OPML file.")
		return
	}
	var entries []opmlEntry
	flattenOPML(doc.Body.Outlines, "", &entries)
	if len(entries) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_opml", "No feeds found in that file.")
		return
	}
	if len(entries) > maxOPMLFeeds {
		entries = entries[:maxOPMLFeeds]
	}

	userID := userFrom(r.Context()).ID
	var (
		mu       sync.Mutex
		imported int
		failed   []string
		wg       sync.WaitGroup
		sem      = make(chan struct{}, opmlWorkers)
	)
	for _, e := range entries {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
			defer cancel()
			var folder *string
			if e.folder != "" {
				folder = &e.folder
			}
			_, err := rd.ingest.AddFeed(ctx, userID, e.url, folder)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed = append(failed, e.url)
				return
			}
			imported++
		}()
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, map[string]any{"imported": imported, "failed": failed})
}

// --- articles ------------------------------------------------------------

type articleDTO struct {
	ID          int64     `json:"id"`
	FeedID      int64     `json:"feedId"`
	FeedTitle   string    `json:"feedTitle"`
	FeedFavicon *string   `json:"feedFavicon"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Author      *string   `json:"author"`
	PublishedAt time.Time `json:"publishedAt"`
	Excerpt     string    `json:"excerpt"`
	ImageURL    *string   `json:"imageUrl"`
	Read        bool      `json:"read"`
	Saved       bool      `json:"saved"`
	TLDR        string    `json:"tldr"`
	Topics      []string  `json:"topics"`
}

func (rd *reader) listArticles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := defaultPage
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 {
		limit = min(v, maxPage)
	}
	params := store.ListArticlesParams{
		UserID:     userFrom(r.Context()).ID,
		SavedOnly:  q.Get("filter") == "saved",
		UnreadOnly: q.Get("filter") == "unread",
		PageSize:   int32(limit + 1), // one extra row tells us whether there is a next page
	}
	if v := q.Get("feed"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid feed id")
			return
		}
		params.FeedID = &id
	}
	if t := strings.TrimSpace(q.Get("topic")); t != "" {
		params.Topic = &t
	}
	if c := q.Get("cursor"); c != "" {
		ts, id, err := decodeCursor(c)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid cursor")
			return
		}
		params.BeforePublished, params.BeforeID = &ts, &id
	}

	rows, err := rd.q.ListArticles(r.Context(), params)
	if err != nil {
		serverError(w, r, "list articles", err)
		return
	}
	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		c := encodeCursor(last.PublishedAt, last.ID)
		next = &c
	}
	out := make([]articleDTO, 0, len(rows))
	for _, a := range rows {
		out = append(out, articleDTO{
			ID: a.ID, FeedID: a.FeedID, FeedTitle: a.FeedTitle, FeedFavicon: a.FeedFavicon, URL: a.Url,
			Title: a.Title, Author: a.Author, PublishedAt: a.PublishedAt, Excerpt: a.Excerpt,
			ImageURL: a.ImageUrl, Read: a.IsRead, Saved: a.Saved, TLDR: a.Tldr, Topics: a.Topics,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"articles": out, "nextCursor": next})
}

func (rd *reader) getArticle(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	a, err := rd.q.GetArticle(r.Context(), store.GetArticleParams{UserID: userFrom(r.Context()).ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "article not found")
		return
	}
	if err != nil {
		serverError(w, r, "get article", err)
		return
	}
	var keyPoints []string
	_ = json.Unmarshal(a.KeyPoints, &keyPoints)
	if keyPoints == nil {
		keyPoints = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"article": map[string]any{
		"ai": map[string]any{
			"status": a.AiStatus, "tldr": a.Tldr, "keyPoints": keyPoints, "topics": a.Topics,
			"sentiment": a.Sentiment, "readingMinutes": a.ReadingMinutes,
		},
		"id": a.ID, "feedId": a.FeedID, "feedTitle": a.FeedTitle, "url": a.Url, "title": a.Title,
		"author": a.Author, "publishedAt": a.PublishedAt, "contentHtml": a.ContentHtml, "summary": a.Summary,
		"imageUrl": a.ImageUrl, "read": a.IsRead, "saved": a.Saved, "rating": a.Rating,
	}})
}

func (rd *reader) setState(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in struct {
		Read   *bool  `json:"read"`
		Saved  *bool  `json:"saved"`
		Rating *int16 `json:"rating"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.Rating != nil && (*in.Rating < -1 || *in.Rating > 1) {
		writeError(w, http.StatusBadRequest, "invalid_request", "rating must be -1, 0 or 1")
		return
	}
	st, err := rd.q.SetArticleState(r.Context(), store.SetArticleStateParams{
		UserID: userFrom(r.Context()).ID, ArticleID: id, Read: in.Read, Saved: in.Saved, Rating: in.Rating,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "article not found")
		return
	}
	if err != nil {
		serverError(w, r, "set state", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"read": st.IsRead, "saved": st.Saved, "rating": st.Rating})
}

func (rd *reader) markAllRead(w http.ResponseWriter, r *http.Request) {
	params := store.MarkFeedReadParams{UserID: userFrom(r.Context()).ID}
	if v := r.URL.Query().Get("feed"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid feed id")
			return
		}
		params.FeedID = &id
	}
	if err := rd.q.MarkFeedRead(r.Context(), params); err != nil {
		serverError(w, r, "mark read", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers -------------------------------------------------------------

func encodeCursor(t time.Time, id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(t.UnixMicro(), 10) + ":" + strconv.FormatInt(id, 10)))
}

func decodeCursor(c string) (time.Time, int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, 0, err
	}
	a, b, ok := strings.Cut(string(raw), ":")
	if !ok {
		return time.Time{}, 0, errors.New("malformed cursor")
	}
	us, err := strconv.ParseInt(a, 10, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	id, err := strconv.ParseInt(b, 10, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	return time.UnixMicro(us).UTC(), id, nil
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid id")
		return 0, false
	}
	return id, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return false
	}
	return true
}

func cleanFolder(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

func serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	slog.ErrorContext(r.Context(), op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "something went wrong")
}

func (rd *reader) listTopics(w http.ResponseWriter, r *http.Request) {
	rows, err := rd.q.ListTopics(r.Context(), userFrom(r.Context()).ID)
	if err != nil {
		serverError(w, r, "list topics", err)
		return
	}
	type topicDTO struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}
	out := make([]topicDTO, 0, len(rows))
	for _, t := range rows {
		out = append(out, topicDTO{Name: t.Topic, Count: t.ArticleCount})
	}
	writeJSON(w, http.StatusOK, map[string]any{"topics": out})
}
