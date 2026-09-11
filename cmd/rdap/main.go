package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/robkerry/rdap/internal/bootstrap"
	"github.com/robkerry/rdap/internal/output"
	rdapclient "github.com/robkerry/rdap/internal/rdap"
)

const version = "1.0.1"

type config struct {
	raw              bool
	rawAll           bool
	nameservers      bool
	csv              bool
	trace            bool
	noColor          bool
	noFollow         bool
	refreshBootstrap bool
	bootstrapURL     string
	maxDepth         int
	timeout          time.Duration
	delay            time.Duration
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, domains, err := parseArgs(args, stdin, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 2
	}

	if len(domains) == 0 {
		fmt.Fprintln(stderr, "Error: no domains supplied")
		return 2
	}

	httpClient := &http.Client{Timeout: cfg.timeout}
	loader := bootstrap.NewLoader(httpClient)
	loader.URL = cfg.bootstrapURL
	loader.Refresh = cfg.refreshBootstrap

	ctx := context.Background()
	registry, err := loader.Load(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 1
	}

	client := rdapclient.NewClient(httpClient, registry)
	client.MaxDepth = cfg.maxDepth
	client.FollowRelated = !cfg.noFollow

	printer := output.New(stdout, stderr)
	printer.Color = !cfg.noColor && isTerminal(stdout)

	results := make([]*rdapclient.Result, 0, len(domains))
	failures := make(map[string]error)

	for i, domain := range domains {
		result, lookupErr := client.LookupDomain(ctx, domain)
		if lookupErr != nil {
			failures[domain] = lookupErr
			if !cfg.csv {
				fmt.Fprintf(stderr, "%s: %v\n", domain, lookupErr)
			}
		} else {
			results = append(results, result)

			if !cfg.csv {
				switch {
				case cfg.rawAll:
					if err := printer.Raw(result, true); err != nil {
						fmt.Fprintln(stderr, err)
					}
				case cfg.raw:
					if err := printer.Raw(result, false); err != nil {
						fmt.Fprintln(stderr, err)
					}
				case cfg.nameservers:
					printer.Nameservers(result, len(domains) > 1)
				default:
					printer.Human(result)
				}

				if cfg.trace {
					printer.Trace(result)
				}
			}
		}

		if i < len(domains)-1 && cfg.delay > 0 {
			time.Sleep(cfg.delay)
			if !cfg.csv && !cfg.raw && !cfg.rawAll && !cfg.nameservers {
				fmt.Fprintln(stdout)
			}
		}
	}

	if cfg.csv {
		if err := output.WriteCSV(stdout, results, failures, time.Now()); err != nil {
			fmt.Fprintln(stderr, "Error writing CSV:", err)
			return 1
		}
	}

	if len(failures) > 0 {
		return 1
	}
	return 0
}

func parseArgs(args []string, stdin io.Reader, stderr io.Writer) (config, []string, error) {
	cfg := config{}
	fs := flag.NewFlagSet("rdap", flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.BoolVar(&cfg.raw, "raw", false, "print the deepest/final RDAP JSON response")
	fs.BoolVar(&cfg.rawAll, "raw-all", false, "print every RDAP response in the referral chain")
	fs.BoolVar(&cfg.nameservers, "nameservers", false, "print nameservers only")
	fs.BoolVar(&cfg.csv, "csv", false, "print CSV (a file argument is treated as a domain list)")
	fs.BoolVar(&cfg.trace, "trace", false, "show RDAP bootstrap/referral traversal on stderr")
	fs.BoolVar(&cfg.noColor, "no-color", false, "disable ANSI colours")
	fs.BoolVar(&cfg.noFollow, "no-follow", false, "do not follow top-level rel=related RDAP referrals")
	fs.BoolVar(&cfg.refreshBootstrap, "refresh-bootstrap", false, "force a fresh IANA bootstrap download")
	fs.StringVar(&cfg.bootstrapURL, "bootstrap-url", bootstrap.DefaultURL, "IANA DNS RDAP bootstrap URL")
	fs.IntVar(&cfg.maxDepth, "max-depth", 4, "maximum related-referral depth")
	fs.DurationVar(&cfg.timeout, "timeout", 30*time.Second, "per-request HTTP timeout")
	fs.DurationVar(&cfg.delay, "delay", 200*time.Millisecond, "delay between domain lookups")

	versionFlag := fs.Bool("version", false, "print version")
	helpFlag := fs.Bool("help", false, "show help")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `rdap %s — direct IANA-bootstrap RDAP client

Usage:
  rdap [options] domain [domain...]
  rdap --csv domains.txt
  cat domains.txt | rdap --csv

Examples:
  rdap example.co.uk
  rdap --trace example.com
  rdap --raw example.com
  rdap --raw-all example.com
  rdap --nameservers example.co.uk
  rdap --csv domains.txt

Options:
`, version)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return cfg, nil, err
	}
	if *versionFlag {
		fmt.Fprintf(fs.Output(), "rdap %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}
	if *helpFlag {
		fs.Usage()
		os.Exit(0)
	}

	if cfg.raw && cfg.rawAll {
		return cfg, nil, errors.New("--raw and --raw-all are mutually exclusive")
	}
	if (cfg.raw || cfg.rawAll) && cfg.csv {
		return cfg, nil, errors.New("--raw/--raw-all cannot be combined with --csv")
	}
	if cfg.nameservers && cfg.csv {
		return cfg, nil, errors.New("--nameservers cannot be combined with --csv")
	}
	if cfg.maxDepth < 0 {
		return cfg, nil, errors.New("--max-depth must be 0 or greater")
	}

	var rawInputs []string
	for _, arg := range fs.Args() {
		// Convenience requested for: rdap --csv domains.txt
		if cfg.csv {
			if info, err := os.Stat(arg); err == nil && !info.IsDir() {
				fileInputs, readErr := readDomainFile(arg)
				if readErr != nil {
					return cfg, nil, readErr
				}
				rawInputs = append(rawInputs, fileInputs...)
				continue
			}
		}
		rawInputs = append(rawInputs, arg)
	}

	if len(rawInputs) == 0 && !isReaderTerminal(stdin) {
		scanner := bufio.NewScanner(stdin)
		for scanner.Scan() {
			rawInputs = append(rawInputs, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return cfg, nil, err
		}
	}

	domains := make([]string, 0, len(rawInputs))
	seen := map[string]struct{}{}
	for _, input := range rawInputs {
		domain, err := normalizeDomain(input)
		if err != nil {
			return cfg, nil, err
		}
		if domain == "" {
			continue
		}
		if _, exists := seen[domain]; exists {
			continue
		}
		seen[domain] = struct{}{}
		domains = append(domains, domain)
	}

	return cfg, domains, nil
}

func normalizeDomain(input string) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" || strings.HasPrefix(value, "#") {
		return "", nil
	}

	if strings.Contains(value, "://") {
		u, err := url.Parse(value)
		if err != nil {
			return "", fmt.Errorf("invalid URL %q: %w", value, err)
		}
		value = u.Hostname()
	} else {
		value = strings.SplitN(value, "/", 2)[0]
		if host, _, found := strings.Cut(value, ":"); found {
			value = host
		}
	}

	value = strings.Trim(strings.ToLower(strings.TrimSpace(value)), ".")
	if value == "" {
		return "", nil
	}

	for _, r := range value {
		if r > 127 {
			return "", fmt.Errorf(
				"%q contains Unicode; use its IDNA A-label/punycode form (xn--...)",
				input,
			)
		}
	}

	if !strings.Contains(value, ".") {
		return "", fmt.Errorf("%q does not look like a fully-qualified domain", input)
	}

	return value, nil
}

func readDomainFile(path string) ([]string, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		out = append(out, scanner.Text())
	}
	return out, scanner.Err()
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

func isReaderTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}
