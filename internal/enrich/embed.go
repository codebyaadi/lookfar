package enrich

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/codebyaadi/feedoku/internal/llm"
	"github.com/codebyaadi/feedoku/internal/store"
)

// Embedder fills in missing article embeddings in batches. It is decoupled
// from summarization so each works without the other.
type Embedder struct {
	q      *store.Queries
	voyage *llm.Voyage
	batch  int
}

func NewEmbedder(q *store.Queries, v *llm.Voyage, batch int) *Embedder {
	return &Embedder{q: q, voyage: v, batch: batch}
}

func (e *Embedder) Enabled() bool { return e.voyage.Enabled() }

// Sweep embeds one batch of articles and returns how many were embedded.
// A *llm.ErrRateLimited error means "try again later".
func (e *Embedder) Sweep(ctx context.Context) (int, error) {
	rows, err := e.q.ListNeedingEmbedding(ctx, int32(e.batch))
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	texts := make([]string, len(rows))
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
		texts[i] = r.Title + "\n\n" + truncate(r.ContentText, 1500)
	}
	vecs, tokens, err := e.voyage.Embed(ctx, texts, "document")
	if err != nil {
		if _, limited := err.(*llm.ErrRateLimited); !limited {
			_ = e.q.BumpEmbedAttempts(ctx, ids)
		}
		return 0, err
	}
	for i, v := range vecs {
		if err := e.q.SaveEmbedding(ctx, store.SaveEmbeddingParams{ArticleID: ids[i], Embedding: vectorLiteral(v)}); err != nil {
			return i, fmt.Errorf("save embedding %d: %w", ids[i], err)
		}
	}
	if err := e.q.AddUsage(ctx, store.AddUsageParams{Scope: "embed", InputTokens: int64(tokens)}); err != nil {
		slog.Warn("record usage", "err", err)
	}
	return len(rows), nil
}

// vectorLiteral formats v in pgvector's text form: [0.1,0.2,...].
func vectorLiteral(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
