// Main package
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/oklog/run"
	"github.com/peterbourgon/ff/v3"
)

const progName = "docker-socket-proxy"

var projectVersion = "dev"

func usageFor(fs *flag.FlagSet) func() {
	return func() {
		fmt.Fprintln(os.Stderr, "USAGE")
		fmt.Fprintf(os.Stderr, "  %s [options]\n", fs.Name())
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "OPTIONS")

		tw := tabwriter.NewWriter(os.Stderr, 0, 2, 2, ' ', 0)
		fmt.Fprintf(tw, "  Flag\tEnv Var\tDescription\n")
		fs.VisitAll(func(f *flag.Flag) {
			var envVar string
			if f.Name != "verbose" && f.Name != "version" {
				envVar = strings.ReplaceAll(strings.ToUpper(f.Name), "-", "_")
			}
			var defValue string
			if f.DefValue != "" {
				defValue = fmt.Sprintf(" (default: %s)", f.DefValue)
			}
			fmt.Fprintf(tw, "  -%s\t%s\t%s%s\n", f.Name, envVar, f.Usage, defValue)
		})
		if err := tw.Flush(); err != nil {
			panic(err)
		}
	}
}

type config struct {
	AllowFilters []*AllowFilter `toml:"allow_filters"`
}

func main() {
	// Command-line arguments
	var (
		apiListen   string
		configFile  string
		socketFile  string
		verbose     bool
		versionFlag bool
	)

	fs := flag.NewFlagSet(progName, flag.ExitOnError)
	fs.StringVar(&apiListen, "api-listen", "127.0.0.1:2375", "Listen address")
	fs.StringVar(&configFile, "config-file", "", "Path to the TOML configuration file")
	fs.StringVar(&socketFile, "socket-file", "/var/run/docker.sock", "Path to the Docker socket file")
	fs.BoolVar(&verbose, "verbose", false, "Be more verbose")
	fs.BoolVar(&versionFlag, "version", false, "Print version information and exit")
	fs.Usage = usageFor(fs)

	if err := ff.Parse(fs, os.Args[1:], ff.WithEnvVarNoPrefix()); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing flags: %v\n", err)
		os.Exit(2)
	}

	if versionFlag {
		fmt.Printf("%s version %s\n", progName, projectVersion)
		os.Exit(0)
	}

	var logger *slog.Logger
	{
		opts := &slog.HandlerOptions{
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey && len(groups) == 0 {
					return slog.Attr{}
				}
				return a
			},
		}
		if verbose {
			opts.Level = slog.LevelDebug
		}
		logger = slog.New(slog.NewTextHandler(os.Stderr, opts))
	}

	var cfg config
	_, err := toml.DecodeFile(configFile, &cfg)
	if err != nil {
		logger.Error("parsing configuration failed", "err", err)
		os.Exit(1)
	}

	var g run.Group

	{
		proxyLogger := logger.With("component", "proxy")
		proxy, err := NewProxy(socketFile, proxyLogger)
		if err != nil {
			proxyLogger.Error("initialization failed", "err", err)
			os.Exit(1)
		}

		filterLogger := logger.With("component", "request_filter")
		ras := make(RequestAccepters, len(cfg.AllowFilters))
		for i, f := range cfg.AllowFilters {
			if err := f.Validate(); err != nil {
				filterLogger.Error("validation failed", "err", err)
				os.Exit(1)
			}
			ras[i] = f
		}
		fh := NewFilteringMiddleware(proxy, ras, filterLogger)

		srv := http.Server{
			Addr:    apiListen,
			Handler: fh,
		}

		apiLogger := logger.With("component", "http_api")
		g.Add(func() error {
			defer apiLogger.Info("shutting down")
			apiLogger.Info("start listening", "addr", apiListen)
			return srv.ListenAndServe()
		}, func(_ error) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				apiLogger.Error("shutdown failed", "err", err)
			}
		})
	}

	g.Add(run.SignalHandler(context.Background(), syscall.SIGINT, syscall.SIGTERM))

	runErr := g.Run()

	if !errors.Is(runErr, run.ErrSignal) {
		logger.Error("running failed", "err", runErr)
		os.Exit(1)
	}

	logger.Info("terminating", "msg", runErr)
}
