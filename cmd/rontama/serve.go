package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/Minalinnski/RonTama/internal/discovery"
	"github.com/Minalinnski/RonTama/internal/net/server"
)

// runServe hosts a Sichuan game on TCP. Empty seats are filled with
// Easy bots after -timeout.
func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 7777, "TCP port to listen on")
	timeout := fs.Duration("timeout", 30*time.Second, "wait this long for joiners before filling with bots")
	announce := fs.Bool("announce", true, "publish via mDNS so 'join' can auto-discover")
	if err := fs.Parse(args); err != nil {
		return err
	}

	addr := ":" + strconv.Itoa(*port)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if *announce {
		closer, err := discovery.Announce("", *port, []string{"rule=sichuan-bloodbattle"})
		if err != nil {
			log.Warn("mDNS announce failed", "err", err)
		} else {
			defer closer.Close()
			log.Info("mDNS announced", "service", discovery.ServiceType, "port", *port)
		}
	}

	cfg := server.Config{Addr: addr, JoinTimeout: *timeout, Log: log}
	if err := server.Run(ctx, cfg); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	fmt.Fprintln(os.Stderr, "round complete; server shutting down.")
	return nil
}

// runJoin connects to a server (auto-discover or explicit address).
// Default is interactive TUI; pass -bot to play as a headless Easy bot
// (useful for testing or filling seats).
func runJoin(args []string) error {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	addr := fs.String("addr", "", "host:port of server (empty = mDNS discover)")
	browseTimeout := fs.Duration("discover", 3*time.Second, "mDNS discovery timeout")
	headless := fs.Bool("bot", false, "join as a headless Easy bot instead of launching the TUI")
	ruleName := fs.String("rule", "sichuan", "rule set the server is hosting (sichuan | riichi)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	target := *addr
	if target == "" {
		log.Info("browsing mDNS for servers", "timeout", *browseTimeout)
		ctx, cancel := context.WithTimeout(context.Background(), *browseTimeout+time.Second)
		defer cancel()
		found, err := discovery.Browse(ctx, *browseTimeout)
		if err != nil {
			return fmt.Errorf("browse: %w", err)
		}
		if len(found) == 0 {
			return fmt.Errorf("no servers found via mDNS — pass -addr host:port")
		}
		fmt.Fprintln(os.Stderr, "discovered:")
		for _, f := range found {
			fmt.Fprintf(os.Stderr, "  %s @ %s  %s\n", f.Name, f.Addr, strings.Join(f.Info, " "))
		}
		target = found[0].Addr
		log.Info("connecting to first match", "addr", target)
	}

	rule, err := pickRule(*ruleName)
	if err != nil {
		return err
	}
	if *headless {
		return joinAsHeadlessBot(target, rule, log)
	}
	return joinAsTUI(target, rule, log)
}
