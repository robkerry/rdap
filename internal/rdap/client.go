package rdap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"rdapcli/internal/bootstrap"
)

type Hop struct {
	Depth       int
	Via         string
	URL         string
	FinalURL    string
	HTTPStatus  int
	Redirects   []string
	Raw         json.RawMessage
	Domain      Domain
}

type Result struct {
	Query string
	Hops  []Hop
}

func (r Result) DeepestHop() *Hop {
	if len(r.Hops) == 0 {
		return nil
	}
	best := &r.Hops[0]
	for i := range r.Hops {
		if r.Hops[i].Depth >= best.Depth {
			best = &r.Hops[i]
		}
	}
	return best
}

type Client struct {
	HTTPClient    *http.Client
	Bootstrap     *bootstrap.Registry
	MaxDepth      int
	FollowRelated bool
	UserAgent     string
}

func NewClient(httpClient *http.Client, reg *bootstrap.Registry) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		HTTPClient:    httpClient,
		Bootstrap:     reg,
		MaxDepth:      4,
		FollowRelated: true,
		UserAgent:     "rdap-cli/1.0",
	}
}

type queueItem struct {
	URL   string
	Depth int
	Via   string
}

func (c *Client) LookupDomain(ctx context.Context, domain string) (*Result, error) {
	if c.Bootstrap == nil {
		return nil, errors.New("RDAP client has no IANA bootstrap registry")
	}
	if c.MaxDepth < 0 {
		c.MaxDepth = 0
	}

	bases, err := c.Bootstrap.URLsForDomain(domain)
	if err != nil {
		return nil, err
	}

	var first Hop
	var firstErr error
	found := false

	for _, base := range bases {
		queryURL, err := domainURL(base, domain)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		hop, err := c.fetchDomain(ctx, queryURL, 0, "IANA bootstrap")
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		first = hop
		found = true
		break
	}

	if !found {
		if firstErr == nil {
			firstErr = errors.New("all IANA-provided RDAP services failed")
		}
		return nil, firstErr
	}

	result := &Result{Query: domain, Hops: []Hop{first}}
	if !c.FollowRelated || c.MaxDepth == 0 {
		return result, nil
	}

	visited := map[string]struct{}{}
	markVisited(visited, first.URL)
	markVisited(visited, first.FinalURL)
	for _, r := range first.Redirects {
		markVisited(visited, r)
	}

	queue := referrals(first, domain, 1)
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.Depth > c.MaxDepth {
			continue
		}
		key := canonicalURL(item.URL)
		if _, ok := visited[key]; ok {
			continue
		}
		visited[key] = struct{}{}

		hop, err := c.fetchDomain(ctx, item.URL, item.Depth, item.Via)
		if err != nil {
			// A broken registrar referral should not make the authoritative
			// registry response unusable. Keep the successful chain we have.
			continue
		}

		result.Hops = append(result.Hops, hop)
		markVisited(visited, hop.FinalURL)
		for _, r := range hop.Redirects {
			markVisited(visited, r)
		}

		if item.Depth < c.MaxDepth {
			for _, next := range referrals(hop, domain, item.Depth+1) {
				if _, ok := visited[canonicalURL(next.URL)]; !ok {
					queue = append(queue, next)
				}
			}
		}
	}

	return result, nil
}

func (c *Client) fetchDomain(ctx context.Context, queryURL string, depth int, via string) (Hop, error) {
	redirects := []string{}
	baseTransport := c.HTTPClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}

	client := &http.Client{
		Transport: baseTransport,
		Timeout:   c.HTTPClient.Timeout,
		CheckRedirect: func(req *http.Request, viaReqs []*http.Request) error {
			if len(viaReqs) >= 10 {
				return errors.New("too many HTTP redirects")
			}
			redirects = append(redirects, req.URL.String())
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
	if err != nil {
		return Hop{}, err
	}
	req.Header.Set("Accept", "application/rdap+json, application/json;q=0.9")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		return Hop{}, fmt.Errorf("%s: %w", queryURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return Hop{}, fmt.Errorf("%s: read response: %w", queryURL, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Hop{}, fmt.Errorf("%s: HTTP %d: %s", queryURL, resp.StatusCode, compactError(body))
	}

	var d Domain
	if err := json.Unmarshal(body, &d); err != nil {
		return Hop{}, fmt.Errorf("%s: invalid RDAP JSON: %w", queryURL, err)
	}

	return Hop{
		Depth:      depth,
		Via:        via,
		URL:        queryURL,
		FinalURL:   resp.Request.URL.String(),
		HTTPStatus: resp.StatusCode,
		Redirects: redirects,
		Raw:        append(json.RawMessage(nil), body...),
		Domain:     d,
	}, nil
}

func referrals(hop Hop, domain string, depth int) []queueItem {
	out := []queueItem{}
	for _, link := range hop.Domain.Links {
		if !strings.EqualFold(link.Rel, "related") || link.Href == "" {
			continue
		}

		u, err := url.Parse(link.Href)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			continue
		}

		// ICANN's gTLD profile requires the top-level registry domain
		// response to link to the registrar's RDAP domain object using
		// rel="related". Avoid following arbitrary "related" web links.
		if !looksLikeDomainObjectURL(u, domain, link.Type) {
			continue
		}

		out = append(out, queueItem{
			URL:   u.String(),
			Depth: depth,
			Via:   `rel="related" referral`,
		})
	}
	return out
}

func looksLikeDomainObjectURL(u *url.URL, domain, mediaType string) bool {
	p := strings.ToLower(u.EscapedPath())
	if strings.Contains(p, "/domain/") {
		return true
	}
	t := strings.ToLower(mediaType)
	if strings.Contains(t, "application/rdap+json") {
		return true
	}

	// Some implementations omit "type" but return the domain name at the
	// end of the referral URL.
	escapedDomain := strings.ToLower(url.PathEscape(domain))
	return strings.HasSuffix(strings.TrimSuffix(p, "/"), "/"+escapedDomain)
}

func domainURL(base, domain string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid RDAP base URL %q: %w", base, err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("unsupported RDAP base URL scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid RDAP base URL %q", base)
	}

	u.Path = strings.TrimSuffix(u.Path, "/") + "/domain/" + url.PathEscape(domain)
	u.RawPath = ""
	return u.String(), nil
}

func compactError(body []byte) string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return "empty response"
	}
	if len(body) > 300 {
		body = body[:300]
	}
	return strings.Join(strings.Fields(string(body)), " ")
}

func canonicalURL(value string) string {
	u, err := url.Parse(value)
	if err != nil {
		return strings.TrimSpace(value)
	}
	u.Fragment = ""
	u.Host = strings.ToLower(u.Host)
	u.Scheme = strings.ToLower(u.Scheme)
	u.Path = path.Clean(u.Path)
	if u.Path == "." {
		u.Path = "/"
	}
	return u.String()
}

func markVisited(seen map[string]struct{}, value string) {
	if value == "" {
		return
	}
	seen[canonicalURL(value)] = struct{}{}
}
