// Package ingest turns feeds into stored articles: adding feeds, polling them
// with conditional GETs, and extracting full text for teaser-only items.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/codebyaadi/feedoku/internal/feed"
	"github.com/codebyaadi/feedoku/internal/store"
)

const (
	pollInterval = 30 * time.Minute
	leaseSeconds = 300 // how long a claimed feed stays out of other workers' reach
	maxBackoff   = 24 * time.Hour
	// Items whose feed content is shorter than this get a readability pass.
	shortContent = 600
	// Cap page downloads per poll so one huge feed can't stall the worker.
	maxExtractPerPoll = 10
	maxItemsPerPoll   = 100
)

type Service struct {
	q       *store.Queries
	fetcher *feed.Fetcher
}

func New(q *store.Queries, f *feed.Fetcher) *Service { return &Service{q: q, fetcher: f} }

// AddFeed resolves rawURL to a feed, stores its first batch of articles and
// subscribes the user. The same feed is shared between users.
func (s *Service) AddFeed(ctx context.Context, userID, rawURL string, folder *string) (store.Feed, error) {
	d, err := s.fetcher.Discover(ctx, rawURL)
	if err != nil {
		return store.Feed{}, err
	}
	f, err := s.q.UpsertFeed(ctx, store.UpsertFeedParams{
		Url:         d.URL,
		SiteUrl:     nonEmpty(d.Parsed.SiteURL),
		Title:       d.Parsed.Title,
		Description: d.Parsed.Description,
		Favicon:     faviconFor(d.Parsed.SiteURL),
	})
	if err != nil {
		return store.Feed{}, err
	}
	if err := s.q.Subscribe(ctx, store.SubscribeParams{UserID: userID, FeedID: f.ID, Folder: folder}); err != nil {
		return store.Feed{}, err
	}
	if f.LastFetchedAt == nil {
		if err := s.storeItems(ctx, f.ID, d.Parsed.Items); err != nil {
			slog.WarnContext(ctx, "initial ingest failed", "feed", f.Url, "err", err)
		}
		_ = s.q.MarkFeedFetched(ctx, store.MarkFeedFetchedParams{
			ID: f.ID, SiteUrl: nonEmpty(d.Parsed.SiteURL), Title: d.Parsed.Title,
			Description: d.Parsed.Description, Favicon: faviconFor(d.Parsed.SiteURL),
			Etag: nonEmpty(d.Resp.ETag), LastModified: nonEmpty(d.Resp.LastModified),
			IntervalSecs: int32(pollInterval.Seconds()),
		})
	}
	return f, nil
}

// PollDue claims up to batch due feeds and refreshes them with limited
// concurrency. It returns how many feeds were polled.
func (s *Service) PollDue(ctx context.Context, batch, concurrency int) (int, error) {
	feeds, err := s.q.ClaimDueFeeds(ctx, store.ClaimDueFeedsParams{LeaseSecs: leaseSeconds, Batch: int32(batch)})
	if err != nil {
		return 0, err
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, f := range feeds {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			s.Poll(ctx, f)
		}()
	}
	wg.Wait()
	return len(feeds), nil
}

// Poll refreshes one feed and records the outcome on the feed row.
func (s *Service) Poll(ctx context.Context, f store.Feed) {
	log := slog.With("feed_id", f.ID, "url", f.Url)
	resp, err := s.fetcher.Get(ctx, f.Url, deref(f.Etag), deref(f.LastModified))
	if errors.Is(err, feed.ErrNotModified) {
		_ = s.q.MarkFeedNotModified(ctx, store.MarkFeedNotModifiedParams{ID: f.ID, IntervalSecs: int32(pollInterval.Seconds())})
		return
	}
	var parsed *feed.Parsed
	if err == nil {
		parsed, err = feed.Parse(resp.Body, resp.FinalURL)
	}
	if err != nil {
		s.fail(ctx, f, err)
		log.Warn("poll failed", "err", err, "errors", f.ErrorCount+1)
		return
	}
	if err := s.storeItems(ctx, f.ID, parsed.Items); err != nil {
		s.fail(ctx, f, err)
		log.Warn("store failed", "err", err)
		return
	}
	if err := s.q.MarkFeedFetched(ctx, store.MarkFeedFetchedParams{
		ID: f.ID, SiteUrl: nonEmpty(parsed.SiteURL), Title: parsed.Title,
		Description: parsed.Description, Favicon: faviconFor(parsed.SiteURL),
		Etag: nonEmpty(resp.ETag), LastModified: nonEmpty(resp.LastModified),
		IntervalSecs: int32(pollInterval.Seconds()),
	}); err != nil {
		log.Error("mark fetched", "err", err)
	}
}

func (s *Service) fail(ctx context.Context, f store.Feed, cause error) {
	backoff := pollInterval << min(f.ErrorCount, 6)
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	msg := cause.Error()
	if err := s.q.MarkFeedFailed(ctx, store.MarkFeedFailedParams{ID: f.ID, LastError: &msg, BackoffSecs: int32(backoff.Seconds())}); err != nil {
		slog.ErrorContext(ctx, "mark feed failed", "err", err)
	}
}

func (s *Service) storeItems(ctx context.Context, feedID int64, items []feed.Item) error {
	if len(items) > maxItemsPerPoll {
		items = items[:maxItemsPerPoll]
	}
	guids := make([]string, len(items))
	for i, it := range items {
		guids[i] = it.GUID
	}
	known, err := s.q.ExistingGUIDs(ctx, store.ExistingGUIDsParams{FeedID: feedID, Guids: guids})
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(known))
	for _, g := range known {
		seen[g] = true
	}

	extracted := 0
	for _, it := range items {
		if seen[it.GUID] {
			continue
		}
		if len(it.ContentText) < shortContent && it.URL != "" && extracted < maxExtractPerPoll {
			extracted++
			if h, t, img, err := s.fetcher.ExtractArticle(ctx, it.URL); err == nil && len(t) > len(it.ContentText) {
				it.ContentHTML, it.ContentText = h, t
				if it.ImageURL == "" {
					it.ImageURL = img
				}
			}
		}
		if err := s.q.InsertArticle(ctx, store.InsertArticleParams{
			FeedID: feedID, Guid: it.GUID, Url: it.URL, Title: it.Title,
			Author: nonEmpty(it.Author), PublishedAt: it.PublishedAt, Summary: it.Summary,
			ContentHtml: it.ContentHTML, ContentText: it.ContentText, ImageUrl: nonEmpty(it.ImageURL),
		}); err != nil {
			return fmt.Errorf("insert %q: %w", it.GUID, err)
		}
	}
	return nil
}

func faviconFor(siteURL string) *string {
	u, err := url.Parse(siteURL)
	if err != nil || u.Host == "" {
		return nil
	}
	s := u.Scheme + "://" + u.Host + "/favicon.ico"
	return &s
}

func nonEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
