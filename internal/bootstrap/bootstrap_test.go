package bootstrap

import "testing"

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
