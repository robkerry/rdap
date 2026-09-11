package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/robkerry/rdap/internal/rdap"
)

type Printer struct {
	Out      io.Writer
	Err      io.Writer
	Color    bool
	Now      func() time.Time
}

func New(out, errOut io.Writer) *Printer {
	return &Printer{
		Out: out,
		Err: errOut,
		Now: time.Now,
	}
}

func (p *Printer) Human(result *rdap.Result) {
	s := rdap.Summarize(result, p.Now())
	bold, cyan, reset := "", "", ""
	if p.Color {
		bold, cyan, reset = "\x1b[1m", "\x1b[36m", "\x1b[0m"
	}

	fmt.Fprintf(p.Out, "%s%s%s\n", bold, s.Domain, reset)
	row := func(label, value string) {
		if value == "" {
			value = "-"
		}
		fmt.Fprintf(p.Out, "%s%-13s%s %s\n", cyan, label+":", reset, value)
	}

	row("Registrar", s.Registrar)
	row("Registered", formatDate(s.Registered))
	row("Updated", formatDate(s.Updated))

	exp := formatDate(s.Expires)
	if s.DaysToExpiry != nil {
		switch {
		case *s.DaysToExpiry < 0:
			exp += fmt.Sprintf(" (%d days ago)", -*s.DaysToExpiry)
		case *s.DaysToExpiry == 0:
			exp += " (today)"
		default:
			exp += fmt.Sprintf(" (%d days)", *s.DaysToExpiry)
		}
	}
	row("Expires", exp)
	row("Status", strings.Join(s.Status, ", "))
	row("Nameservers", strings.Join(s.Nameservers, ", "))
	row("Handle", s.Handle)
	row("RDAP", s.RDAPURL)
	row("Hops", fmt.Sprintf("%d", s.Hops))
}

func (p *Printer) Trace(result *rdap.Result) {
	for i, hop := range result.Hops {
		fmt.Fprintf(
			p.Err,
			"[%d] depth=%d via=%s HTTP=%d\n    requested: %s\n    final:     %s\n",
			i+1, hop.Depth, hop.Via, hop.HTTPStatus, hop.URL, hop.FinalURL,
		)
		for _, redirect := range hop.Redirects {
			fmt.Fprintf(p.Err, "    redirect:  %s\n", redirect)
		}
	}
}

func (p *Printer) Raw(result *rdap.Result, all bool) error {
	if all {
		type item struct {
			Depth    int             `json:"depth"`
			Via      string          `json:"via"`
			URL      string          `json:"url"`
			FinalURL string          `json:"final_url"`
			Response json.RawMessage `json:"response"`
		}
		items := make([]item, 0, len(result.Hops))
		for _, hop := range result.Hops {
			items = append(items, item{
				Depth: hop.Depth, Via: hop.Via, URL: hop.URL,
				FinalURL: hop.FinalURL, Response: hop.Raw,
			})
		}
		enc := json.NewEncoder(p.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}

	hop := result.DeepestHop()
	if hop == nil {
		return fmt.Errorf("no RDAP response")
	}
	var pretty any
	if err := json.Unmarshal(hop.Raw, &pretty); err != nil {
		return err
	}
	enc := json.NewEncoder(p.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(pretty)
}

func (p *Printer) Nameservers(result *rdap.Result, includeDomain bool) {
	s := rdap.Summarize(result, p.Now())
	for _, ns := range s.Nameservers {
		if includeDomain {
			fmt.Fprintf(p.Out, "%s\t%s\n", s.Domain, ns)
		} else {
			fmt.Fprintln(p.Out, ns)
		}
	}
}

func WriteCSV(w io.Writer, results []*rdap.Result, errs map[string]error, now time.Time) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := []string{
		"domain", "registrar", "registered", "updated", "expires",
		"days_to_expiry", "status", "nameservers", "rdap_url", "hops", "error",
	}
	if err := cw.Write(header); err != nil {
		return err
	}

	for _, result := range results {
		s := rdap.Summarize(result, now)
		days := ""
		if s.DaysToExpiry != nil {
			days = fmt.Sprintf("%d", *s.DaysToExpiry)
		}
		row := []string{
			s.Domain,
			s.Registrar,
			s.Registered,
			s.Updated,
			s.Expires,
			days,
			strings.Join(s.Status, "|"),
			strings.Join(s.Nameservers, "|"),
			s.RDAPURL,
			fmt.Sprintf("%d", s.Hops),
			"",
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}

	for domain, err := range errs {
		row := []string{domain, "", "", "", "", "", "", "", "", "0", err.Error()}
		if writeErr := cw.Write(row); writeErr != nil {
			return writeErr
		}
	}

	return cw.Error()
}

func formatDate(value string) string {
	if value == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.UTC().Format("2006-01-02 15:04:05 UTC")
}
