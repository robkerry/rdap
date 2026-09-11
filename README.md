# rdap

A small cross-platform RDAP command-line client written in Go.

Unlike clients that proxy lookups through a third-party service, `rdap`:

1. downloads the official IANA DNS RDAP bootstrap registry;
2. performs the RFC 9224 longest label-wise match for the queried domain;
3. queries the authoritative registry RDAP service directly;
4. follows HTTP redirects;
5. follows top-level RDAP `rel="related"` referrals (for example, registry → registrar);
6. stops safely using a visited-URL set and a configurable maximum referral depth.

The IANA bootstrap registry is cached using HTTP cache metadata / `Expires`
where available, with a 24-hour fallback.

## Install

### Homebrew

```bash
brew install robkerry/rdap/rdap
```

### Scoop (Windows)

```powershell
scoop bucket add rdap https://github.com/robkerry/scoop-rdap
scoop install rdap
```

### Install script (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/robkerry/rdap/main/scripts/install.sh | bash
```

The script downloads the latest GitHub release, checks the SHA-256, and
installs into `/usr/local/bin` if it is writable, otherwise `~/.local/bin`.
Override the destination with `RDAP_INSTALL_DIR`.

### Install script (Windows PowerShell)

```powershell
irm https://raw.githubusercontent.com/robkerry/rdap/main/scripts/install.ps1 | iex
```

The script downloads the latest GitHub release, checks the SHA-256, and
installs into `%LOCALAPPDATA%\rdap`, then adds that directory to your user
`PATH`. Override the destination with `$env:RDAP_INSTALL_DIR`.

### Go

Requires Go 1.22+.

```bash
go install github.com/robkerry/rdap/cmd/rdap@latest
```

### Manual download

Release archives and `SHA256SUMS` are published at
https://github.com/robkerry/rdap/releases

```bash
# macOS Apple Silicon example
curl -fsSL -O https://github.com/robkerry/rdap/releases/latest/download/rdap_1.0.1_darwin_arm64.tar.gz
tar -xzf rdap_1.0.1_darwin_arm64.tar.gz
sudo install -m 0755 rdap /usr/local/bin/rdap
```

Windows users who are not using Scoop or the install script should
download `rdap_*_windows_amd64.zip` or `rdap_*_windows_arm64.zip` and put
`rdap.exe` on `PATH`.

Then:

```bash
rdap example.co.uk
```

## Build

Requires Go 1.22+.

```bash
go test ./...
go build -o rdap ./cmd/rdap
```

## Usage

```text
rdap example.co.uk
rdap --trace example.com
rdap --raw example.com
rdap --raw-all example.com
rdap --nameservers example.co.uk
rdap example.com example.net example.org
rdap --csv domains.txt
cat domains.txt | rdap --csv
```

### `--trace`

Shows the actual service chain on stderr:

```text
[1] depth=0 via=IANA bootstrap HTTP=200
    requested: https://registry.example/rdap/domain/example.com
    final:     https://registry.example/rdap/domain/example.com
[2] depth=1 via=rel="related" referral HTTP=200
    requested: https://registrar.example/rdap/domain/example.com
    final:     https://registrar.example/rdap/domain/example.com
```

### Raw data

`--raw` prints the deepest successfully-retrieved RDAP object.

`--raw-all` emits every successful RDAP response in the chain, together with
its depth, source, requested URL and final URL.

### CSV

If `--csv` is enabled and an argument is an existing file, it is treated as a
newline-delimited list of domains:

```bash
rdap --csv domains.txt > results.csv
```

The columns are:

```text
domain,registrar,registered,updated,expires,days_to_expiry,status,nameservers,rdap_url,hops,error
```

## IANA bootstrap and referrals

The client follows RFC 9224 rather than depending on `rdap.org`. For domain
queries, IANA publishes one or more base RDAP URLs. `rdap` prefers HTTPS and
tries those services until one succeeds.

For gTLDs, current ICANN RDAP implementation requirements specify a top-level
`rel="related"` link from a registry's domain response to the registrar's
RDAP domain object when the registrar RDAP URL is available. `rdap` follows
such links when they look like RDAP domain-object URLs.

Referral traversal is bounded by:

- a visited-URL set;
- `--max-depth` (default 4);
- normal HTTP redirect limit (10);
- per-request timeout (default 30 seconds).

A broken downstream referral does not discard a valid upstream registry
response.

## Options

```text
--raw
--raw-all
--nameservers
--csv
--trace
--no-color
--no-follow
--refresh-bootstrap
--bootstrap-url URL
--max-depth N
--timeout 30s
--delay 200ms
--version
--help
```

## IDNs

The binary deliberately has no third-party Go dependencies. Unicode IDN input
therefore needs to be supplied as its IDNA A-label / punycode form, for
example `xn--...`.

## Release builds

A GitHub Actions workflow is included. Creating a tag such as `v1.0.0`
produces archives for:

- macOS Apple Silicon (`darwin/arm64`)
- macOS Intel (`darwin/amd64`)
- Linux x64 (`linux/amd64`)
- Linux ARM64 (`linux/arm64`)
- Windows x64 (`windows/amd64`)
- Windows ARM64 (`windows/arm64`)

The release workflow also generates SHA-256 checksums and attaches
`scripts/install.sh` and `scripts/install.ps1`. After tagging a new
version, update `Formula/rdap.rb` and `scoop/rdap.json` in this repo,
https://github.com/robkerry/homebrew-rdap, and
https://github.com/robkerry/scoop-rdap.

## Licence

MIT.
