package net_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/net/client"
	"github.com/Minalinnski/RonTama/internal/net/server"
	"github.com/Minalinnski/RonTama/internal/rules/sichuan"
)

// TestEndToEnd_OneClientThreeBots starts a server, dials a single
// client (headless Easy bot), and lets the round play out. Validates
// that the wire protocol handshake works and the round completes
// without hanging.
func TestEndToEnd_OneClientThreeBots(t *testing.T) {
	port := freePort(t)
	addr := "127.0.0.1:" + strconv.Itoa(port)
	silent := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = server.Run(ctx, server.Config{
			Addr:        addr,
			JoinTimeout: 800 * time.Millisecond,
			Log:         silent,
		})
	}()

	// Give the listener a moment to come up before dialing.
	if !waitForPort(addr, 2*time.Second) {
		t.Fatal("server didn't start listening in time")
	}

	rule := sichuan.New()
	bot := easy.New("client-bot")
	d := client.NewHeadlessDecider(bot, rule)
	c, err := client.Dial(addr, d, silent)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	if err := c.Run(); err != nil {
		t.Fatalf("client run: %v", err)
	}

	wg.Wait()
}

// freePort opens a random free port and immediately closes the listener.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// waitForPort polls until something accepts TCP at addr or timeout elapses.
func waitForPort(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
