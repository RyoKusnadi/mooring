package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ryokusnadi/mooring/internal/bench"
)

// runBench is `mooring bench`. A real run shows what it will do and reads "yes" from in.
func runBench(args []string, in io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("mooring bench", flag.ContinueOnError)
	flags.SetOutput(stderr)
	agent := flags.String("agent", "claude", "agent to measure")
	account := flags.String("account", "", "account id (default: the agent's default account)")
	tasks := flags.String("tasks", "all", "comma-separated tasks: "+strings.Join(bench.TaskNames(), ", ")+", or all")
	modes := flags.String("modes", "off,on", "comma-separated modes: off, on")
	budget := flags.Int64("budget-tokens", 3_000_000, "tokens for the whole run; a task over a tenth of it is cancelled")
	dry := flags.Bool("dry", false, "use the stand-in agent: no real agent, account or tokens")
	standIn := flags.String("stand-in", "", "with -dry: the stand-in agent program (go test -c -o fakeagent ./internal/chat)")
	dataDir := flags.String("data", "", "Mooring data folder (default: the user config folder)")
	benchDir := flags.String("bench-dir", "bench", "folder with fixture, check and fix")
	tmp := flags.String("tmp", "", "folder for the task copies (default: the system temporary folder)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unknown argument %q", flags.Arg(0))
	}
	if *dry && *standIn == "" {
		return errors.New("-dry needs -stand-in: build one with go test -c -o fakeagent ./internal/chat")
	}
	dir, err := resolveDataDir(*dataDir)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	_, err = bench.Run(ctx, bench.Options{
		Agent: *agent, Account: *account, Tasks: split(*tasks), Modes: split(*modes), Budget: *budget, Dry: *dry,
		DataDir: dir, BenchDir: *benchDir, TmpDir: *tmp, Exe: *standIn, In: in, Out: stdout,
		Log: slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
	})
	return err
}

func split(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
