package enrich

import (
	"strings"
	"testing"
)

func TestParseResultHandlesFencesAndNormalizes(t *testing.T) {
	text := "Sure! ```json\n" + `{"tldr":"  Go 1.27 reuses connections.  ","key_points":["a","", "b"],
"topics":["programming","Made Up","AI & ML","programming","Security","Space"],
"entities":["Go","  "],"sentiment":"POSITIVE"}` + "\n```"
	r, err := ParseResult(text)
	if err != nil {
		t.Fatal(err)
	}
	if r.TLDR != "Go 1.27 reuses connections." {
		t.Errorf("tldr = %q", r.TLDR)
	}
	if got := strings.Join(r.KeyPoints, "|"); got != "a|b" {
		t.Errorf("key points = %q", got)
	}
	// canonical casing, de-duplicated, unknown dropped, capped at 3
	if got := strings.Join(r.Topics, "|"); got != "Programming|AI & ML|Security" {
		t.Errorf("topics = %q", got)
	}
	if r.Sentiment != "positive" || len(r.Entities) != 1 {
		t.Errorf("sentiment/entities = %q %v", r.Sentiment, r.Entities)
	}
}

func TestParseResultFallbacks(t *testing.T) {
	r, err := ParseResult(`{"tldr":"x","key_points":["y"],"topics":["nonsense"],"sentiment":"mixed"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Topics) != 1 || r.Topics[0] != "Other" || r.Sentiment != "neutral" {
		t.Errorf("fallbacks = %+v", r)
	}
}

func TestParseResultRejectsBadOutput(t *testing.T) {
	for name, in := range map[string]string{
		"no json":       "I cannot help with that",
		"broken json":   `{"tldr": "x",`,
		"empty tldr":    `{"tldr":" ","key_points":["a"]}`,
		"no key points": `{"tldr":"x","key_points":[]}`,
	} {
		if _, err := ParseResult(in); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestParseResultTruncatesLongTLDR(t *testing.T) {
	r, err := ParseResult(`{"tldr":"` + strings.Repeat("é", 900) + `","key_points":["a"]}`)
	if err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(r.TLDR)); n > 400 {
		t.Errorf("tldr has %d runes", n)
	}
}

func TestVectorLiteral(t *testing.T) {
	if got := vectorLiteral([]float32{0.5, -1, 0.25}); got != "[0.5,-1,0.25]" {
		t.Errorf("got %s", got)
	}
}
