package rdap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rdapcli/internal/bootstrap"
)

func TestFollowsTopLevelRelatedRegistrarReferral(t *testing.T) {
	var registrar *httptest.Server
	registrar = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		_, _ = w.Write([]byte(`{
			"objectClassName":"domain",
			"ldhName":"example.test",
			"status":["active"],
			"events":[{"eventAction":"expiration","eventDate":"2030-01-01T00:00:00Z"}]
		}`))
	}))
	defer registrar.Close()

	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		_, _ = w.Write([]byte(`{
			"objectClassName":"domain",
			"ldhName":"example.test",
			"links":[{
				"value":"` + r.URL.String() + `",
				"rel":"related",
				"href":"` + registrar.URL + `/domain/example.test",
				"type":"application/rdap+json"
			}]
		}`))
	}))
	defer registry.Close()

	c := NewClient(registry.Client(), &bootstrap.Registry{
		Services: []bootstrap.Service{
			{Entries: []string{"test"}, URLs: []string{registry.URL + "/"}},
		},
	})

	result, err := c.LookupDomain(context.Background(), "example.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hops) != 2 {
		t.Fatalf("expected 2 hops, got %d", len(result.Hops))
	}
	if result.Hops[1].Depth != 1 {
		t.Fatalf("expected referral depth 1, got %d", result.Hops[1].Depth)
	}
}

func TestDoesNotFollowArbitraryRelatedHTML(t *testing.T) {
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		_, _ = w.Write([]byte(`{
			"objectClassName":"domain",
			"ldhName":"example.test",
			"links":[{
				"value":"https://example.test/",
				"rel":"related",
				"href":"https://example.test/privacy",
				"type":"text/html"
			}]
		}`))
	}))
	defer registry.Close()

	c := NewClient(registry.Client(), &bootstrap.Registry{
		Services: []bootstrap.Service{
			{Entries: []string{"test"}, URLs: []string{registry.URL + "/"}},
		},
	})

	result, err := c.LookupDomain(context.Background(), "example.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hops) != 1 {
		t.Fatalf("expected 1 hop, got %d", len(result.Hops))
	}
}

func TestDomainURL(t *testing.T) {
	got, err := domainURL("https://rdap.example/base/", "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "/base/domain/example.com") {
		t.Fatalf("unexpected URL: %s", got)
	}
}
