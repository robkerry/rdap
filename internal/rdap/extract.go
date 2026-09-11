package rdap

import (
	"strings"
	"time"
)

type Summary struct {
	Domain       string
	Handle       string
	Registrar    string
	Registered   string
	Updated      string
	Expires      string
	DaysToExpiry *int
	Status       []string
	Nameservers  []string
	RDAPURL      string
	Hops         int
}

func Summarize(result *Result, now time.Time) Summary {
	s := Summary{
		Domain: result.Query,
		Hops:   len(result.Hops),
	}

	// Prefer the deepest referral response, but fill missing fields from
	// earlier authoritative responses.
	for depth := maxDepth(result); depth >= 0; depth-- {
		for i := len(result.Hops) - 1; i >= 0; i-- {
			hop := result.Hops[i]
			if hop.Depth != depth {
				continue
			}
			d := hop.Domain

			if s.Domain == "" {
				s.Domain = firstNonEmpty(d.LDHName, d.UnicodeName)
			}
			if s.Handle == "" {
				s.Handle = d.Handle
			}
			if s.Registrar == "" {
				s.Registrar = RegistrarName(d)
			}
			if s.Registered == "" {
				s.Registered = d.EventDate("registration")
			}
			if s.Updated == "" {
				s.Updated = d.EventDate("last changed", "last update of RDAP database")
			}
			if s.Expires == "" {
				s.Expires = d.EventDate("expiration")
			}
			if len(s.Status) == 0 && len(d.Status) > 0 {
				s.Status = append([]string(nil), d.Status...)
			}
			if len(s.Nameservers) == 0 && len(d.Nameservers) > 0 {
				for _, ns := range d.Nameservers {
					name := firstNonEmpty(ns.LDHName, ns.UnicodeName)
					if name != "" {
						s.Nameservers = append(s.Nameservers, name)
					}
				}
			}
			if s.RDAPURL == "" {
				s.RDAPURL = firstNonEmpty(hop.FinalURL, hop.URL)
			}
		}
	}

	s.Nameservers = uniqueStrings(s.Nameservers)
	s.Status = uniqueStrings(s.Status)

	if expiry, ok := ParseTime(s.Expires); ok {
		days := int(expiry.Sub(now).Hours() / 24)
		if expiry.After(now) && now.Add(time.Duration(days)*24*time.Hour).Before(expiry) {
			days++
		}
		s.DaysToExpiry = &days
	}

	return s
}

func maxDepth(result *Result) int {
	max := 0
	for _, hop := range result.Hops {
		if hop.Depth > max {
			max = hop.Depth
		}
	}
	return max
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, value := range in {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}
