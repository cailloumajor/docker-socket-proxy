// Main package
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/oklog/run"
	"github.com/peterbourgon/ff/v3"
)

const progName = "docker-socket-proxy"

var projectVersion = "dev"

func usageFor(fs *flag.FlagSet, out io.Writer) func() {
	return func() {
		fmt.Fprintln(out, "USAGE")
		fmt.Fprintf(out, "  %s [options]\n", fs.Name())
		fmt.Fprintln(out)
		fmt.Fprintln(out, "OPTIONS")

		tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
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

func main() {
	// Command-line arguments
	var (
		socketFile  string
		apiListen   string
		verbose     bool
		versionFlag bool
	)

	fs := flag.NewFlagSet(progName, flag.ExitOnError)
	fs.StringVar(&socketFile, "socket-file", "/var/run/docker.sock", "Path to the Docker socket file")
	fs.StringVar(&apiListen, "listen", "127.0.0.1:2375", "Listen address")
	fs.BoolVar(&verbose, "verbose", false, "Be more verbose")
	fs.BoolVar(&versionFlag, "version", false, "Print version information and exit")
	fs.Usage = usageFor(fs, os.Stderr)

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

	var g run.Group

	{
		proxyLogger := log.With(logger, "component", "proxy")

		proxy, err := NewProxy(socketFile, proxyLogger)
		if err != nil {
			level.Error(proxyLogger).Log("during", "initialization", "err", err)
			os.Exit(1)
		}

		srv := http.Server{
			Addr:    apiListen,
			Handler: proxy,
		}

		g.Add(func() error {
			defer level.Info(proxyLogger).Log("status", "shutting down")
			level.Info(proxyLogger).Log("status", "start listening", "addr", apiListen)
			return srv.ListenAndServe()
		}, func(err error) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				level.Error(proxyLogger).Log("during", "shutdown", "err", err)
			}
		})
	}

	g.Add(run.SignalHandler(context.Background(), syscall.SIGINT, syscall.SIGTERM))

	runErr := g.Run()

	var se run.SignalError
	if !errors.As(runErr, &se) {
		level.Error(logger).Log("exit", "error")
		os.Exit(1)
	}

	level.Info(logger).Log("exit", runErr)
}
