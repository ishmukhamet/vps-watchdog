package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ishmukhamet/vps-watchdog/internal/agent"
	"github.com/ishmukhamet/vps-watchdog/internal/config"
	"github.com/ishmukhamet/vps-watchdog/internal/event"
	"github.com/ishmukhamet/vps-watchdog/internal/notifier/telegram"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC | log.Lmsgprefix)
	log.SetPrefix("vps-watchdog: ")

	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		printUsage(os.Stderr)
		return errors.New("missing command")
	}

	switch args[1] {
	case "agent":
		return runAgent(args[2:])
	case "test-notify":
		return runTestNotify(args[2:])
	case "validate-config":
		return runValidateConfig(args[2:])
	case "version":
		fmt.Printf("vps-watchdog %s commit=%s built=%s\n", version, commit, date)
		return nil
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown command %q", args[1])
	}
}

func printUsage(out *os.File) {
	fmt.Fprintf(out, `vps-watchdog monitors a Linux VPS and sends Telegram alerts.

Usage:
  vps-watchdog agent --config /etc/vps-watchdog/config.yaml
  vps-watchdog test-notify --config /etc/vps-watchdog/config.yaml
  vps-watchdog validate-config --config /etc/vps-watchdog/config.yaml
  vps-watchdog version
`)
}

func runAgent(args []string) error {
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	configPath := fs.String("config", "/etc/vps-watchdog/config.yaml", "path to config file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		return err
	}

	n, err := telegram.NewFromConfig(cfg.Telegram)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runner := agent.New(cfg, n, log.Default())
	return runner.Run(ctx)
}

func runValidateConfig(args []string) error {
	fs := flag.NewFlagSet("validate-config", flag.ContinueOnError)
	configPath := fs.String("config", "/etc/vps-watchdog/config.yaml", "path to config file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		return err
	}

	fmt.Printf("config ok: host_id=%q login=%t sudo=%t load=%t\n", cfg.HostID, cfg.Login.Enabled, cfg.Sudo.Enabled, cfg.Load.Enabled)
	return nil
}

func runTestNotify(args []string) error {
	fs := flag.NewFlagSet("test-notify", flag.ContinueOnError)
	configPath := fs.String("config", "/etc/vps-watchdog/config.yaml", "path to config file")
	message := fs.String("message", "", "optional custom test message")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		return err
	}

	n, err := telegram.NewFromConfig(cfg.Telegram)
	if err != nil {
		return err
	}

	body := strings.TrimSpace(*message)
	e := event.Event{
		Time:     time.Now().UTC(),
		Host:     cfg.HostID,
		Type:     event.TypeAgent,
		Severity: event.SeverityInfo,
		Title:    "vps-watchdog test notification",
		Fields: map[string]string{
			"version": version,
			"commit":  commit,
		},
	}
	if body != "" {
		e.Fields["message"] = body
	}
	body = event.FormatTelegramHTML(e)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := n.Send(ctx, body); err != nil {
		return err
	}
	fmt.Println("test notification sent")
	return nil
}
