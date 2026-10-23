package source

import (
	"strings"
	"testing"
)

const pricingPage = `<!doctype html><html><head><title> Acme Pricing </title>
<style>.x{color:red}</style><script>var ticking = Date.now()</script></head>
<body>
<div id="cookie-banner">We use cookies. Accept all</div>
<nav><a href="/">Home</a></nav>
<main>
  <h1>Pricing</h1>
  <p>Updated 5 minutes ago</p>
  <table>
    <tr><th>Plan</th><th>Price</th></tr>
    <tr><td>Pro</td><td>$49 / month</td></tr>
  </table>
  <div class="promo">Sale ends 2026-10-31T23:59:00Z</div>
  <p>Last refreshed at 14:32:07</p>
</main>
<footer>© Acme</footer>
</body></html>`

func TestNormalizePageStripsNoise(t *testing.T) {
	title, text, err := NormalizePage([]byte(pricingPage), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if title != "Acme Pricing" {
		t.Errorf("title = %q", title)
	}
	for _, bad := range []string{"cookies", "ticking", "color:red", "minutes ago", "14:32", "2026-10-31T23", "Accept all"} {
		if strings.Contains(text, bad) {
			t.Errorf("text still contains %q:\n%s", bad, text)
		}
	}
	for _, want := range []string{"Pricing", "Plan | Price", "Pro | $49 / month", "Sale ends", "Home", "© Acme"} {
		if !strings.Contains(text, want) {
			t.Errorf("text is missing %q:\n%s", want, text)
		}
	}
}

func TestNormalizePageIsStableAcrossTimestamps(t *testing.T) {
	ticked := strings.ReplaceAll(pricingPage, "5 minutes ago", "9 minutes ago")
	ticked = strings.ReplaceAll(ticked, "14:32:07", "15:01:44")
	_, t1, _ := NormalizePage([]byte(pricingPage), Config{})
	_, t2, _ := NormalizePage([]byte(ticked), Config{})
	if t1 != t2 {
		t.Fatalf("a ticking timestamp changed the content:\n%s\n---\n%s", t1, t2)
	}
	if d := LineDiff(t1, t2); d.Added+d.Removed != 0 {
		t.Fatalf("unexpected diff: %+v", d)
	}
}

func TestNormalizePageKeepTimestamps(t *testing.T) {
	_, text, err := NormalizePage([]byte(pricingPage), Config{KeepTimestamps: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "5 minutes ago") || !strings.Contains(text, "14:32:07") {
		t.Errorf("timestamps should be kept:\n%s", text)
	}
}

func TestNormalizePageDetectsRealChange(t *testing.T) {
	changed := strings.ReplaceAll(pricingPage, "$49", "$59")
	_, t1, _ := NormalizePage([]byte(pricingPage), Config{})
	_, t2, _ := NormalizePage([]byte(changed), Config{})
	d := LineDiff(t1, t2)
	if d.Added != 1 || d.Removed != 1 {
		t.Fatalf("want one replaced line, got %+v", d)
	}
}

func TestNormalizePageSelectorAndIgnores(t *testing.T) {
	_, text, err := NormalizePage([]byte(pricingPage), Config{
		Selector:        "main",
		IgnoreSelectors: []string{"h1"},
		IgnorePatterns:  []string{`^Sale`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "Home") || strings.Contains(text, "© Acme") {
		t.Errorf("selector should scope to <main>:\n%s", text)
	}
	if strings.Contains(text, "Pricing") || strings.Contains(text, "Sale ends") {
		t.Errorf("ignore rules not applied:\n%s", text)
	}
	if !strings.Contains(text, "Pro | $49 / month") {
		t.Errorf("content lost:\n%s", text)
	}
}

func TestNormalizePageSelectorMatchesNothing(t *testing.T) {
	_, _, err := NormalizePage([]byte(pricingPage), Config{Selector: "#does-not-exist"})
	if err == nil || !strings.Contains(err.Error(), "matched nothing") {
		t.Fatalf("err = %v", err)
	}
}
