package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestURLsForDomainLongestMatch(t *testing.T) {
	reg := &Registry{
		Services: []Service{
			{Entries: []string{"uk"}, URLs: []string{"https://uk.example/"}},
			{Entries: []string{"co.uk"}, URLs: []string{"https://co-uk.example/"}},
		},
	}

	got, err := reg.URLsForDomain("Example.CO.UK.")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "https://co-uk.example/" {
		t.Fatalf("got %#v", got)
	}
}

func TestURLsPreferHTTPS(t *testing.T) {
	reg := &Registry{
		Services: []Service{
			{Entries: []string{"com"}, URLs: []string{
				"http://rdap.example/",
				"https://rdap.example/",
			}},
		},
	}
	got, err := reg.URLsForDomain("example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "https://rdap.example/" {
		t.Fatalf("expected HTTPS first, got %#v", got)
	}
}

func TestLoadIgnoresCacheFromADifferentBootstrapURL(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer bad.Close()

	dir := t.TempDir()
	cachePath := filepath.Join(dir, "iana-dns.json")
	body, err := json.Marshal(map[string]any{
		"version":     "1.0",
		"publication": "2026-01-01T00:00:00Z",
		"services":    []any{[]any{[]string{"com"}, []string{"https://rdap.example/"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	cache, err := json.Marshal(cacheFile{
		URL:       DefaultURL,
		FetchedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		Body:      body,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, cache, 0o600); err != nil {
		t.Fatal(err)
	}

	loader := &Loader{
		URL:        bad.URL + "/not-rdap.json",
		HTTPClient: bad.Client(),
		CachePath:  cachePath,
	}
	if _, err := loader.Load(context.Background()); err == nil {
		t.Fatal("expected custom bootstrap URL failure, used a different URL's cache")
	}
}

func TestLoadUsesFreshCacheForTheSameURL(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "iana-dns.json")
	body, err := json.Marshal(map[string]any{
		"version":     "1.0",
		"publication": "2026-01-01T00:00:00Z",
		"services":    []any{[]any{[]string{"com"}, []string{"https://rdap.example/"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	cache, err := json.Marshal(cacheFile{
		URL:       DefaultURL,
		FetchedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		Body:      body,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, cache, 0o600); err != nil {
		t.Fatal(err)
	}

	loader := &Loader{
		URL:       DefaultURL,
		CachePath: cachePath,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("should not fetch when same-URL cache is fresh")
			return nil, nil
		})},
	}
	reg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	urls, err := reg.URLsForDomain("example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 1 || urls[0] != "https://rdap.example/" {
		t.Fatalf("got %#v", urls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
