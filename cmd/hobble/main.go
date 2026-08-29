// Command hobble is a fault-injection reverse proxy for testing how
// resilient a client is to network/service problems.
package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/mangrisano/hobble/internal/proxy"
	"github.com/spf13/cobra"
)

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
	cmd.MarkFlagRequired("target")

	return cmd
}
