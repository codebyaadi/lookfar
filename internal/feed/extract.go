package feed

import (
	"bytes"
	"context"
	"net/url"

	"github.com/go-shiori/go-readability"
)

// ExtractArticle downloads pageURL and returns the readable main content
// (sanitized HTML and plain text). Used when the feed only carries a teaser.
func (f *Fetcher) ExtractArticle(ctx context.Context, pageURL string) (htmlOut, text, image string, err error) {
	resp, err := f.Get(ctx, pageURL, "", "")
	if err != nil {
		return "", "", "", err
	}
	u, err := url.Parse(resp.FinalURL)
	if err != nil {
		return "", "", "", err
	}
	art, err := readability.FromReader(bytes.NewReader(resp.Body), u)
	if err != nil {
		return "", "", "", err
	}
	return SanitizeHTML(art.Content), PlainText(art.TextContent), art.Image, nil
}
