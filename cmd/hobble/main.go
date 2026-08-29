// Command hobble is a fault-injection reverse proxy for testing how
// resilient a client is to network/service problems.
package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/mangrisano/hobble/internal/proxy"
	"github.com/spf13/cobra"
)

// version is overwritten at build time via -ldflags "-X main.version=...".
var version = "dev"

// resolveVersion prefers an ldflags-injected version, then the module version
// embedded by `go install`, then the VCS commit of a local build.
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			rev := s.Value
			if len(rev) > 7 {
				rev = rev[:7]
			}
			return "dev (" + rev + ")"
		}
	}
	return version
}

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// newRootCmd builds hobble's cobra command, wiring flags to proxy.Config.
func newRootCmd() *cobra.Command {
	var (
		target  string
		listen  string
		latency string
		status  []string
		drop    float64
	)

	cmd := &cobra.Command{
		Use:          "hobble",
		Short:        "Fault-injection reverse proxy",
		Version:      resolveVersion(),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			latencyRange, err := proxy.ParseLatencyRange(latency)
			if err != nil {
				return err
			}

			statusRules := make([]proxy.StatusRule, 0, len(status))
			for _, s := range status {
				rule, err := proxy.ParseStatusRule(s)
				if err != nil {
					return err
				}
				statusRules = append(statusRules, rule)
			}

			reverseProxy, err := proxy.NewReverseProxy(target)
			if err != nil {
				return err
			}

			handler := proxy.WithFaults(reverseProxy, statusRules, latencyRange, drop)

			fmt.Printf("hobble listening on %s, forwarding to %s\n", listen, target)
			return http.ListenAndServe(listen, handler)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&target, "target", "t", "", "target URL to proxy requests to (required)")
	f.StringVarP(&listen, "listen", "l", ":8080", "address to listen on")
	f.StringVar(&latency, "latency", "0", "latency to inject, fixed (200ms) or range (200ms-800ms)")
	f.StringArrayVar(&status, "status", nil, "status code to inject with probability, e.g. 500=0.1 (repeatable)")
	f.Float64Var(&drop, "drop", 0, "probability of dropping the connection")
	f.BoolP("version", "v", false, "print version and exit")
	cmd.MarkFlagRequired("target")

	cmd.SetVersionTemplate("hobble {{.Version}}\n")

	return cmd
}
