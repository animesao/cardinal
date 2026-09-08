//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// exitFunc is the process-termination seam for every CLI command in this
// package. Production code paths call exitFunc(1) exactly where they used
// to call os.Exit(1), so runtime behaviour is unchanged; tests override
// exitFunc to capture the exit code instead of terminating the test binary:
//
//	old := exitFunc
//	exitFunc = func(code int) { got = code }
//	defer func() { exitFunc = old }()
var exitFunc = os.Exit

// failf prints a uniform error line to stderr and terminates the command
// with exit code 1 (via exitFunc, so it stays stubbable in tests).
func failf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	exitFunc(1)
}

// mustParse parses args with a stdlib FlagSet that uses ContinueOnError so
// the flag package can never terminate the process itself. It preserves the
// historical ExitOnError user experience:
//
//   - `-h`/`--help` prints the flag usage to stderr and exits 0;
//   - any other parse failure prints "Error parsing <name> options: ..."
//     to stderr and exits 1 (previously the flag package exited 2 with its
//     own message; the code is unified to 1 so cobra wrappers, scripts and
//     tests observe a single failure convention).
func mustParse(fs *flag.FlagSet, args []string, name string) {
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fs.SetOutput(os.Stderr)
			fs.Usage()
			exitFunc(0)
			return
		}
		fmt.Fprintf(os.Stderr, "Error parsing %s options: %v\n", name, err)
		exitFunc(1)
	}
}
