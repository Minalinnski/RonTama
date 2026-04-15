package main

import (
	"log/slog"

	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/net/client"
	"github.com/Minalinnski/RonTama/internal/rules/sichuan"
)

// joinAsHeadlessBot dials the server and runs the protocol with an
// Easy bot driving the responses.
func joinAsHeadlessBot(addr string, log *slog.Logger) error {
	rule := sichuan.New()
	bot := easy.New("net-easy")
	decider := client.NewHeadlessDecider(bot, rule)

	c, err := client.Dial(addr, decider, log)
	if err != nil {
		return err
	}
	defer c.Close()
	return c.Run()
}
