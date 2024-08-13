// Main package
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
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
				envVar = strings.Replace(strings.ToUpper(f.Name), "-", "_", -1)
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
		fmt.Fprintf(os.Stdout, "%s version %s\n", progName, projectVersion)
		os.Exit(0)
	}

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
		var logLevel level.Option
		if verbose {
			logLevel = level.AllowDebug()
		} else {
			logLevel = level.AllowInfo()
		}
		logger = level.NewFilter(logger, logLevel)
	}

	var cfg config
	_, err := toml.DecodeFile(configFile, &cfg)
	if err != nil {
		level.Error(logger).Log("during", "decoding configuration", "err", err)
		os.Exit(1)
	}

	var g run.Group

	{
		proxyLogger := log.With(logger, "component", "proxy")
		proxy, err := NewProxy(socketFile, proxyLogger)
		if err != nil {
			level.Error(proxyLogger).Log("during", "initialization", "err", err)
			os.Exit(1)
		}

		filterLogger := log.With(logger, "component", "request_filter")
		ras := make(RequestAccepters, len(cfg.AllowFilters))
		for i, f := range cfg.AllowFilters {
			if err := f.Validate(); err != nil {
				level.Error(filterLogger).Log("during", "filter validation", "err", err)
				os.Exit(1)
			}
			ras[i] = f
		}
		fh := NewFilteringMiddleware(proxy, ras, filterLogger)

		srv := http.Server{
			Addr:    apiListen,
			Handler: fh,
		}

		apiLogger := log.With(logger, "component", "http_api")
		g.Add(func() error {
			defer level.Info(apiLogger).Log("status", "shutting down")
			level.Info(apiLogger).Log("status", "start listening", "addr", apiListen)
			return srv.ListenAndServe()
		}, func(_ error) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				level.Error(apiLogger).Log("during", "shutdown", "err", err)
			}
		})
	}

	g.Add(run.SignalHandler(context.Background(), syscall.SIGINT, syscall.SIGTERM))

	runErr := g.Run()

	var se run.SignalError
	if !errors.As(runErr, &se) {
		level.Error(logger).Log("status", "program end", "err", runErr)
		os.Exit(1)
	}

	level.Info(logger).Log("status", "program end", "msg", runErr)
}
