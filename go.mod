module cardinal

go 1.26.0

// 1.26.9 is the first release carrying the net/http, net/textproto and
// crypto/tls fixes for GO-2026-6603/6605/6607/6608/6610/6611/6612/6613/6617.
// The `go` line stays at 1.26.0 so the language version does not move; this
// only raises the *toolchain* floor so a default GOTOOLCHAIN=auto build on
// 1.26.0-1.26.8 self-upgrades instead of silently linking a vulnerable
// standard library into the released binary.
toolchain go1.26.9

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/creack/pty v1.1.24
	github.com/prometheus/client_golang v1.25.0
	github.com/spf13/cobra v1.10.2
	golang.org/x/sys v0.49.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)
