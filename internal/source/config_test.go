package source

import "testing"

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		cfg     Config
		wantErr bool
	}{
		{"empty page", KindPage, Config{}, false},
		{"good selector", KindPage, Config{Selector: "main .price, #plans"}, false},
		{"bad selector", KindPage, Config{Selector: "div[["}, true},
		{"bad ignore selector", KindPage, Config{IgnoreSelectors: []string{"p:::"}}, true},
		{"bad regex", KindPage, Config{IgnorePatterns: []string{"(unclosed"}}, true},
		{"negative threshold", KindRSS, Config{MinChangeChars: -1}, true},
		{"too many items", KindRSS, Config{MaxItems: 1000}, true},
		{"unknown kind", "ftp", Config{}, true},
		{"selector ignored for rss", KindRSS, Config{Selector: "div[["}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(tt.kind); (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCanonicalURL(t *testing.T) {
	tests := []struct {
		kind, in, want string
		wantErr        bool
	}{
		{KindPage, " https://example.com/pricing#plans ", "https://example.com/pricing", false},
		{KindRSS, "http://example.com/feed.xml", "http://example.com/feed.xml", false},
		{KindGitHub, "https://github.com/golang/go/releases/tag/go1.26", "https://github.com/golang/go", false},
		{KindGitHub, "golang/go", "https://github.com/golang/go", false},
		{KindGitHub, "https://github.com/jackc/pgx.git", "https://github.com/jackc/pgx", false},
		{KindGitHub, "https://gitlab.com/a/b", "", true},
		{KindGitHub, "https://github.com/onlyowner", "", true},
		{KindPage, "ftp://example.com/file", "", true},
		{KindPage, "not a url", "", true},
		{KindPage, "https://", "", true},
	}
	for _, tt := range tests {
		got, err := CanonicalURL(tt.kind, tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("CanonicalURL(%s, %q) = %q, %v; want %q (err %v)", tt.kind, tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}
