package source

import (
	"bytes"
	"errors"
	"regexp"
	"strings"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const maxContentBytes = 512 << 10

// Elements and attributes that never carry watchable content, plus common
// cookie and consent banners, which change independently of the page itself.
var defaultIgnore = mustGroup(
	`script, style, noscript, svg, iframe, template, canvas, [hidden], [aria-hidden="true"], ` +
		`[role="dialog"], [role="alert"], [role="status"], ` +
		`[id*="cookie" i], [class*="cookie" i], [id*="consent" i], [class*="consent" i], [class*="gdpr" i]`)

var blockTags = map[atom.Atom]bool{
	atom.P: true, atom.Div: true, atom.Section: true, atom.Article: true, atom.Li: true,
	atom.Ul: true, atom.Ol: true, atom.H1: true, atom.H2: true, atom.H3: true, atom.H4: true,
	atom.H5: true, atom.H6: true, atom.Tr: true, atom.Br: true, atom.Table: true, atom.Header: true,
	atom.Footer: true, atom.Nav: true, atom.Main: true, atom.Aside: true, atom.Blockquote: true,
	atom.Pre: true, atom.Hr: true, atom.Form: true, atom.Dd: true, atom.Dt: true, atom.Dl: true,
	atom.Figure: true, atom.Figcaption: true, atom.Body: true, atom.Details: true, atom.Summary: true,
}

var timestampRes = []*regexp.Regexp{
	regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(:\d{2})?(\.\d+)?(Z|[+-]\d{2}:?\d{2})?\b`),
	regexp.MustCompile(`(?i)\b\d{1,2}:\d{2}(:\d{2})?\s?(am|pm)?\b`),
	regexp.MustCompile(`(?i)\b(last\s+)?(updated|modified|posted|published|refreshed)?\s*\d+\s+(second|minute|hour|day|week|month|year)s?\s+ago\b`),
	regexp.MustCompile(`(?i)\b(just now|moments? ago|yesterday|today)\b`),
}

func mustGroup(sel string) cascadia.SelectorGroup {
	g, err := cascadia.ParseGroup(sel)
	if err != nil {
		panic(err)
	}
	return g
}

// NormalizePage turns an HTML document into stable lines of visible text and
// returns the page title. Whitespace is collapsed, ignored regions and noise
// are removed, and (unless cfg.KeepTimestamps) clock and date text is stripped.
func NormalizePage(body []byte, cfg Config) (title, text string, err error) {
	root, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	title = pageTitle(root)

	skip := map[*html.Node]bool{}
	for _, n := range cascadia.QueryAll(root, defaultIgnore) {
		skip[n] = true
	}
	for _, s := range cfg.IgnoreSelectors {
		g, err := cascadia.ParseGroup(s)
		if err != nil {
			return "", "", err
		}
		for _, n := range cascadia.QueryAll(root, g) {
			skip[n] = true
		}
	}

	scope := []*html.Node{findBody(root)}
	if cfg.Selector != "" {
		g, err := cascadia.ParseGroup(cfg.Selector)
		if err != nil {
			return "", "", err
		}
		scope = cascadia.QueryAll(root, g)
		if len(scope) == 0 {
			return title, "", errors.New("selector matched nothing on the page")
		}
	}

	var lines []string
	for _, n := range scope {
		w := &textWalker{skip: skip}
		w.walk(n)
		w.flush()
		lines = append(lines, w.lines...)
	}

	patterns := make([]*regexp.Regexp, 0, len(cfg.IgnorePatterns))
	for _, p := range cfg.IgnorePatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return "", "", err
		}
		patterns = append(patterns, re)
	}
	return title, joinLines(cleanLines(lines, cfg.KeepTimestamps, patterns)), nil
}

func cleanLines(lines []string, keepTimestamps bool, ignore []*regexp.Regexp) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if !keepTimestamps {
			for _, re := range timestampRes {
				l = re.ReplaceAllString(l, "")
			}
		}
		l = strings.TrimSpace(spaceRe.ReplaceAllString(l, " "))
		l = strings.Trim(l, " |-:·")
		if l == "" {
			continue
		}
		dropped := false
		for _, re := range ignore {
			if re.MatchString(l) {
				dropped = true
				break
			}
		}
		if !dropped {
			out = append(out, truncate(l, maxLineLength))
		}
	}
	return out
}

func joinLines(lines []string) string {
	var b strings.Builder
	for i, l := range lines {
		if b.Len()+len(l)+1 > maxContentBytes {
			b.WriteString("\n[content truncated]")
			return b.String()
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(l)
	}
	return b.String()
}

type textWalker struct {
	skip  map[*html.Node]bool
	cur   strings.Builder
	lines []string
}

func (w *textWalker) flush() {
	if s := strings.TrimSpace(w.cur.String()); s != "" {
		w.lines = append(w.lines, s)
	}
	w.cur.Reset()
}

func (w *textWalker) walk(n *html.Node) {
	if w.skip[n] {
		return
	}
	switch n.Type {
	case html.TextNode:
		w.cur.WriteString(n.Data)
		return
	case html.CommentNode, html.DoctypeNode:
		return
	}
	block := n.Type == html.ElementNode && blockTags[n.DataAtom]
	cell := n.Type == html.ElementNode && (n.DataAtom == atom.Td || n.DataAtom == atom.Th)
	if block {
		w.flush()
	}
	if cell && strings.TrimSpace(w.cur.String()) != "" {
		w.cur.WriteString(" | ")
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		w.walk(c)
	}
	if block {
		w.flush()
	}
}

func pageTitle(root *html.Node) string {
	var title string
	var find func(*html.Node)
	find = func(n *html.Node) {
		if title != "" {
			return
		}
		if n.Type == html.ElementNode && n.DataAtom == atom.Title && n.FirstChild != nil {
			title = strings.TrimSpace(spaceRe.ReplaceAllString(n.FirstChild.Data, " "))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(root)
	return truncate(title, 200)
}

func findBody(root *html.Node) *html.Node {
	var body *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		if body != nil {
			return
		}
		if n.Type == html.ElementNode && n.DataAtom == atom.Body {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(root)
	if body == nil {
		return root
	}
	return body
}
