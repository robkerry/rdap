package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const DefaultURL = "https://data.iana.org/rdap/dns.json"

type Service struct {
	Entries []string
	URLs    []string
}

type Registry struct {
	Version     string
	Publication string
	Description string
	Services    []Service
}

type rawRegistry struct {
	Version     string            `json:"version"`
	Publication string            `json:"publication"`
	Description string            `json:"description"`
	Services    []json.RawMessage `json:"services"`
}

type cacheFile struct {
	URL          string          `json:"url,omitempty"`
	FetchedAt    time.Time       `json:"fetched_at"`
	ExpiresAt    time.Time       `json:"expires_at"`
	ETag         string          `json:"etag,omitempty"`
	LastModified string          `json:"last_modified,omitempty"`
	Body         json.RawMessage `json:"body"`
}

type Loader struct {
	URL        string
	HTTPClient *http.Client
	CachePath  string
	Refresh    bool
}

func NewLoader(client *http.Client) *Loader {
	cacheDir, err := os.UserCacheDir()
	cachePath := ""
	if err == nil && cacheDir != "" {
		cachePath = filepath.Join(cacheDir, "rdap-cli", "iana-dns.json")
	}

	return &Loader{
		URL:        DefaultURL,
		HTTPClient: client,
		CachePath:  cachePath,
	}
}

func Parse(data []byte) (*Registry, error) {
	var raw rawRegistry
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse IANA bootstrap JSON: %w", err)
	}

	reg := &Registry{
		Version:     raw.Version,
		Publication: raw.Publication,
		Description: raw.Description,
	}

	for i, item := range raw.Services {
		var parts [][]string
		if err := json.Unmarshal(item, &parts); err != nil {
			return nil, fmt.Errorf("parse IANA service %d: %w", i, err)
		}
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid IANA service %d: expected 2 arrays", i)
		}
		reg.Services = append(reg.Services, Service{
			Entries: parts[0],
			URLs:    parts[1],
		})
	}

	return reg, nil
}

func (l *Loader) Load(ctx context.Context) (*Registry, error) {
	if l.URL == "" {
		l.URL = DefaultURL
	}
	if l.HTTPClient == nil {
		l.HTTPClient = &http.Client{Timeout: 20 * time.Second}
	}

	var cached *cacheFile
	if l.CachePath != "" {
		if c, err := readCache(l.CachePath); err == nil && cacheURL(c) == l.URL {
			cached = c
			if !l.Refresh && time.Now().Before(c.ExpiresAt) && len(c.Body) > 0 {
				return Parse(c.Body)
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "rdap-cli/1.0")

	if cached != nil && !l.Refresh {
		if cached.ETag != "" {
			req.Header.Set("If-None-Match", cached.ETag)
		}
		if cached.LastModified != "" {
			req.Header.Set("If-Modified-Since", cached.LastModified)
		}
	}

	resp, err := l.HTTPClient.Do(req)
	if err != nil {
		if cached != nil && len(cached.Body) > 0 {
			return Parse(cached.Body)
		}
		return nil, fmt.Errorf("download IANA bootstrap registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified && cached != nil {
		cached.URL = l.URL
		cached.FetchedAt = time.Now()
		cached.ExpiresAt = expiryFromHeaders(resp.Header, cached.FetchedAt)
		_ = writeCache(l.CachePath, cached)
		return Parse(cached.Body)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if cached != nil && len(cached.Body) > 0 {
			return Parse(cached.Body)
		}
		return nil, fmt.Errorf("IANA bootstrap registry returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("read IANA bootstrap registry: %w", err)
	}

	reg, err := Parse(body)
	if err != nil {
		return nil, err
	}

	if l.CachePath != "" {
		now := time.Now()
		_ = writeCache(l.CachePath, &cacheFile{
			URL:          l.URL,
			FetchedAt:    now,
			ExpiresAt:    expiryFromHeaders(resp.Header, now),
			ETag:         resp.Header.Get("ETag"),
			LastModified: resp.Header.Get("Last-Modified"),
			Body:         body,
		})
	}

	return reg, nil
}

func (r *Registry) URLsForDomain(domain string) ([]string, error) {
	domain = strings.Trim(strings.ToLower(strings.TrimSpace(domain)), ".")
	if domain == "" {
		return nil, errors.New("empty domain")
	}

	bestLabels := -1
	var urls []string

	for _, service := range r.Services {
		for _, entry := range service.Entries {
			entry = strings.Trim(strings.ToLower(strings.TrimSpace(entry)), ".")
			if entry == "" {
				continue
			}
			if domain != entry && !strings.HasSuffix(domain, "."+entry) {
				continue
			}

			labels := strings.Count(entry, ".") + 1
			if labels > bestLabels {
				bestLabels = labels
				urls = append([]string(nil), service.URLs...)
			} else if labels == bestLabels {
				urls = append(urls, service.URLs...)
			}
		}
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("no IANA RDAP bootstrap service found for %q", domain)
	}

	urls = dedupe(urls)
	sort.SliceStable(urls, func(i, j int) bool {
		ih := strings.HasPrefix(strings.ToLower(urls[i]), "https://")
		jh := strings.HasPrefix(strings.ToLower(urls[j]), "https://")
		if ih == jh {
			return i < j
		}
		return ih && !jh
	})

	return urls, nil
}

func cacheURL(c *cacheFile) string {
	if c == nil || c.URL == "" {
		return DefaultURL
	}
	return c.URL
}

func expiryFromHeaders(h http.Header, now time.Time) time.Time {
	cacheControl := h.Get("Cache-Control")
	for _, part := range strings.Split(cacheControl, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(strings.ToLower(part), "max-age=") {
			continue
		}
		seconds, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.ToLower(part), "max-age=")))
		if err == nil && seconds > 0 {
			return now.Add(time.Duration(seconds) * time.Second)
		}
	}

	if value := h.Get("Expires"); value != "" {
		if t, err := http.ParseTime(value); err == nil && t.After(now) {
			return t
		}
	}

	return now.Add(24 * time.Hour)
}

func readCache(path string) (*cacheFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c cacheFile
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func writeCache(path string, c *cacheFile) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
