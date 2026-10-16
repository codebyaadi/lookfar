// Package enrich turns article text into shared AI output (summary, topics,
// embeddings). Output is stored once per article, not once per user.
package enrich

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/codebyaadi/feedoku/internal/jobs"
	"github.com/codebyaadi/feedoku/internal/llm"
	"github.com/codebyaadi/feedoku/internal/store"
)

const (
	KindEnrichArticle = "enrich_article"

	maxPromptChars = 12000 // ~3k tokens of article text
	// Below this many words there is nothing worth summarizing; skip the LLM call.
	minWordsToSummarize = 60
	wordsPerMinute      = 220
)

// Taxonomy is the fixed topic list the model must choose from, so tags stay
// consistent enough to build smart folders from.
var Taxonomy = []string{
	"AI & ML", "Programming", "Security", "Startups & Business", "Science", "Technology",
	"Hardware", "Space", "Climate & Energy", "Health", "Politics", "World News",
	"Economy & Finance", "Culture & Media", "Gaming", "Design", "Productivity", "Other",
}

type Result struct {
	TLDR      string
	KeyPoints []string
	Topics    []string
	Entities  []string
	Sentiment string
}

// Completer is the part of llm.Chain the enricher needs.
type Completer interface {
	Complete(ctx context.Context, req llm.Request) (*llm.Response, error)
}

type Enricher struct {
	q     *store.Queries
	llm   Completer
	model string
}

func New(q *store.Queries, c Completer) *Enricher { return &Enricher{q: q, llm: c} }

// Handle is the jobs.Handler for KindEnrichArticle.
func (e *Enricher) Handle(ctx context.Context, job store.Job) error {
	var p struct {
		ArticleID int64 `json:"article_id"`
	}
	if err := json.Unmarshal(job.Payload, &p); err != nil || p.ArticleID == 0 {
		return fmt.Errorf("bad payload: %s", job.Payload)
	}
	err := e.enrich(ctx, p.ArticleID)
	var rl *llm.ErrRateLimited
	switch {
	case err == nil:
		return nil
	case errors.As(err, &rl):
		return jobs.Defer(rl.RetryAfter, err)
	case errors.Is(err, pgx.ErrNoRows):
		return nil // article was deleted
	}
	if job.Attempts >= job.MaxAttempts {
		if mErr := e.q.MarkEnrichmentFailed(context.WithoutCancel(ctx), p.ArticleID); mErr != nil {
			slog.Error("mark enrichment failed", "article", p.ArticleID, "err", mErr)
		}
	}
	return err
}

func (e *Enricher) enrich(ctx context.Context, articleID int64) error {
	a, err := e.q.GetArticleForEnrichment(ctx, articleID)
	if err != nil {
		return err
	}
	words := len(strings.Fields(a.Body))
	minutes := max(1, (words+wordsPerMinute-1)/wordsPerMinute)

	if words < minWordsToSummarize {
		return e.save(ctx, articleID, Result{Topics: []string{"Other"}}, minutes, "")
	}

	res, resp, err := e.summarize(ctx, a.Title, a.Body)
	if err != nil {
		return err
	}
	return e.save(ctx, articleID, *res, minutes, resp.Model)
}

func (e *Enricher) save(ctx context.Context, id int64, r Result, minutes int, model string) error {
	kp, _ := json.Marshal(nonNil(r.KeyPoints))
	ent, _ := json.Marshal(nonNil(r.Entities))
	var sentiment *string
	if r.Sentiment != "" {
		sentiment = &r.Sentiment
	}
	return e.q.SaveEnrichment(ctx, store.SaveEnrichmentParams{
		ArticleID: id, Tldr: r.TLDR, KeyPoints: kp, Topics: nonNil(r.Topics), Entities: ent,
		Sentiment: sentiment, ReadingMinutes: int32(minutes), Model: model,
	})
}

func (e *Enricher) summarize(ctx context.Context, title, body string) (*Result, *llm.Response, error) {
	msgs := []llm.Message{
		{Role: "system", Content: "You write concise, factual summaries of articles for a news reader. Reply with one JSON object and nothing else."},
		{Role: "user", Content: buildPrompt(title, body)},
	}
	var lastErr error
	// One retry on malformed output, with the failure fed back to the model.
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := e.llm.Complete(ctx, llm.Request{Messages: msgs, MaxTokens: 1500, JSON: true})
		if err != nil {
			return nil, nil, err
		}
		e.recordUsage(ctx, resp)
		res, perr := ParseResult(resp.Text)
		if perr == nil {
			return res, resp, nil
		}
		lastErr = perr
		msgs = append(msgs,
			llm.Message{Role: "assistant", Content: resp.Text},
			llm.Message{Role: "user", Content: "That was not valid: " + perr.Error() + ". Reply again with only the corrected JSON object."})
	}
	return nil, nil, fmt.Errorf("model returned invalid output twice: %w", lastErr)
}

func (e *Enricher) recordUsage(ctx context.Context, r *llm.Response) {
	if err := e.q.AddUsage(ctx, store.AddUsageParams{
		Scope: "enrich", InputTokens: int64(r.InputTokens), OutputTokens: int64(r.OutputTokens),
	}); err != nil {
		slog.Warn("record usage", "err", err)
	}
}

func buildPrompt(title, body string) string {
	if utf8.RuneCountInString(body) > maxPromptChars {
		body = string([]rune(body)[:maxPromptChars])
	}
	return fmt.Sprintf(`Summarize the article below.

Return JSON with exactly these keys:
{
  "tldr": "one or two sentences, at most 280 characters, no clickbait",
  "key_points": ["2 to 4 short, specific takeaways"],
  "topics": ["1 to 3 values chosen ONLY from the allowed list"],
  "entities": ["up to 5 notable people, companies or products mentioned"],
  "sentiment": "positive | neutral | negative"
}

Allowed topics: %s

Title: %s

Article:
%s`, strings.Join(Taxonomy, ", "), title, body)
}

// ParseResult extracts and validates the model's JSON. It is lenient about
// wrapping (code fences, preamble) and strict about content.
func ParseResult(text string) (*Result, error) {
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return nil, errors.New("no JSON object found")
	}
	var raw struct {
		TLDR      string   `json:"tldr"`
		KeyPoints []string `json:"key_points"`
		Topics    []string `json:"topics"`
		Entities  []string `json:"entities"`
		Sentiment string   `json:"sentiment"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	r := &Result{TLDR: truncateRunes(strings.TrimSpace(raw.TLDR), 400)}
	if r.TLDR == "" {
		return nil, errors.New(`"tldr" is empty`)
	}
	for _, kp := range raw.KeyPoints {
		if kp = strings.TrimSpace(kp); kp != "" && len(r.KeyPoints) < 5 {
			r.KeyPoints = append(r.KeyPoints, truncateRunes(kp, 220))
		}
	}
	if len(r.KeyPoints) == 0 {
		return nil, errors.New(`"key_points" is empty`)
	}
	r.Topics = canonicalTopics(raw.Topics)
	for _, en := range raw.Entities {
		if en = strings.TrimSpace(en); en != "" && len(r.Entities) < 8 {
			r.Entities = append(r.Entities, truncateRunes(en, 60))
		}
	}
	switch s := strings.ToLower(strings.TrimSpace(raw.Sentiment)); s {
	case "positive", "neutral", "negative":
		r.Sentiment = s
	default:
		r.Sentiment = "neutral"
	}
	return r, nil
}

func canonicalTopics(in []string) []string {
	byLower := make(map[string]string, len(Taxonomy))
	for _, t := range Taxonomy {
		byLower[strings.ToLower(t)] = t
	}
	var out []string
	seen := map[string]bool{}
	for _, t := range in {
		c, ok := byLower[strings.ToLower(strings.TrimSpace(t))]
		if ok && !seen[c] && len(out) < 3 {
			seen[c] = true
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return []string{"Other"}
	}
	return out
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
