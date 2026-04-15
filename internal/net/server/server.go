// Package server hosts a RonTama game over TCP.
//
// A Server listens on a port; the first 4 incoming connections are
// assigned seats 0..3 in order. After a configurable JoinTimeout, any
// unfilled seats are taken by Easy bots (so a single player can solo
// against bots over the same protocol). Once all seats are decided,
// the round runs to completion. Server then exits.
//
// The protocol is JSON-line over plain TCP. The handshake is:
//
//	Server → Client: Hello{seat, rule}
//	Server pushes StateUpdate after every public event.
//	Server sends AskDingque/AskDraw/AskCall when the player's turn comes.
//	Client replies with AnswerDingque/AnswerDraw/AnswerCall.
//	Server sends RoundEnd at the end.
package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Minalinnski/RonTama/internal/ai/easy"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/net/proto"
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/rules/sichuan"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// Config holds server tunables.
type Config struct {
	Addr        string        // ":7777"
	JoinTimeout time.Duration // wait this long for clients before filling with bots
	Log         *slog.Logger
}

// Run starts the server and blocks until the round finishes (or ctx is cancelled).
func Run(ctx context.Context, cfg Config) error {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	rule := sichuan.New()

	lc := &net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.Addr, err)
	}
	defer ln.Close()
	cfg.Log.Info("server listening", "addr", ln.Addr())

	conns := make([]net.Conn, 0, game.NumPlayers)
	connCh := make(chan net.Conn, game.NumPlayers)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			select {
			case connCh <- c:
			default:
				_ = c.Close() // overflow
			}
		}
	}()

	timeout := cfg.JoinTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := time.After(timeout)
	cfg.Log.Info("waiting for joiners", "timeout", timeout)

awaitLoop:
	for len(conns) < game.NumPlayers {
		select {
		case c := <-connCh:
			conns = append(conns, c)
			cfg.Log.Info("client joined", "seat", len(conns)-1, "remote", c.RemoteAddr())
		case <-deadline:
			cfg.Log.Info("join timeout reached, filling with bots", "seats_taken", len(conns))
			break awaitLoop
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Build players list. Remote connections become netPlayers; empty seats become Easy bots.
	var players [game.NumPlayers]game.Player
	netPlayers := make([]*netPlayer, len(conns))
	for i, c := range conns {
		np := newNetPlayer(c, i, rule, cfg.Log)
		netPlayers[i] = np
		players[i] = np
		if err := np.sendHello(); err != nil {
			cfg.Log.Warn("hello send failed", "seat", i, "err", err)
		}
	}
	for i := len(conns); i < game.NumPlayers; i++ {
		players[i] = easy.New(fmt.Sprintf("bot-%d", i))
	}

	// Multicast observer pushes StateUpdate to every remote client.
	obs := newMulticastObserver(netPlayers)

	res, err := game.RunRoundWithObserver(rule, players, 0, cfg.Log, obs)

	// Send RoundEnd to every remote client.
	for _, np := range netPlayers {
		if np == nil {
			continue
		}
		if sendErr := np.send(proto.KindRoundEnd, proto.RoundEnd{Result: res}); sendErr != nil {
			cfg.Log.Warn("round_end send failed", "seat", np.seat, "err", sendErr)
		}
		_ = np.conn.Close()
	}
	return err
}

// netPlayer is a game.Player backed by a TCP connection.
type netPlayer struct {
	conn net.Conn
	seat int
	rule rules.RuleSet
	log  *slog.Logger

	mu  sync.Mutex
	enc *bufio.Writer
	dec *json.Decoder
}

func newNetPlayer(c net.Conn, seat int, rule rules.RuleSet, log *slog.Logger) *netPlayer {
	return &netPlayer{
		conn: c, seat: seat, rule: rule, log: log,
		enc: bufio.NewWriter(c),
		dec: json.NewDecoder(bufio.NewReader(c)),
	}
}

func (np *netPlayer) Name() string { return fmt.Sprintf("net-%d", np.seat) }

// ChooseExchange3 implements game.Player by relaying to the client.
func (np *netPlayer) ChooseExchange3(view game.PlayerView) [3]tile.Tile {
	body := proto.AskExchange3{OwnHand: view.OwnHand.Concealed}
	if err := np.send(proto.KindAskExchange3, body); err != nil {
		np.log.Warn("ask_exchange3 send failed", "err", err)
		return [3]tile.Tile{}
	}
	env, err := np.recv()
	if err != nil || env.Kind != proto.KindAnswerExchange3 {
		return [3]tile.Tile{}
	}
	var ans proto.AnswerExchange3
	if err := proto.DecodeBody(env, &ans); err != nil {
		return [3]tile.Tile{}
	}
	return ans.Tiles
}

func (np *netPlayer) sendHello() error {
	return np.send(proto.KindHello, proto.Hello{Seat: np.seat, Rule: np.rule.Name()})
}

func (np *netPlayer) send(kind string, body any) error {
	np.mu.Lock()
	defer np.mu.Unlock()
	line, err := proto.Encode(kind, body)
	if err != nil {
		return err
	}
	if _, err := np.enc.Write(line); err != nil {
		return err
	}
	if err := np.enc.WriteByte('\n'); err != nil {
		return err
	}
	return np.enc.Flush()
}

func (np *netPlayer) recv() (proto.Envelope, error) {
	np.mu.Lock()
	defer np.mu.Unlock()
	var env proto.Envelope
	if err := np.dec.Decode(&env); err != nil {
		return env, err
	}
	return env, nil
}

func (np *netPlayer) ChooseDingque(view game.PlayerView) tile.Suit {
	body := proto.AskDingque{OwnHand: view.OwnHand.Concealed}
	if err := np.send(proto.KindAskDingque, body); err != nil {
		np.log.Warn("ask_dingque failed", "err", err)
		return tile.SuitMan
	}
	env, err := np.recv()
	if err != nil {
		np.log.Warn("ask_dingque recv failed", "err", err)
		return tile.SuitMan
	}
	if env.Kind != proto.KindAnswerDingque {
		np.log.Warn("unexpected reply", "kind", env.Kind)
		return tile.SuitMan
	}
	var ans proto.AnswerDingque
	if err := proto.DecodeBody(env, &ans); err != nil {
		return tile.SuitMan
	}
	return ans.Suit
}

func (np *netPlayer) OnDraw(view game.PlayerView) game.DrawAction {
	body := proto.AskDraw{
		OwnHand:  view.OwnHand.Concealed,
		JustDrew: deref(view.JustDrew),
		Dingque:  view.Dingque[view.Seat],
	}
	if err := np.send(proto.KindAskDraw, body); err != nil {
		return fallbackDiscard(view)
	}
	env, err := np.recv()
	if err != nil {
		return fallbackDiscard(view)
	}
	if env.Kind != proto.KindAnswerDraw {
		return fallbackDiscard(view)
	}
	var ans proto.AnswerDraw
	if err := proto.DecodeBody(env, &ans); err != nil {
		return fallbackDiscard(view)
	}
	return ans.Action
}

func (np *netPlayer) OnCallOpportunity(view game.PlayerView, discarded tile.Tile, from int, opps []game.Call) game.Call {
	body := proto.AskCall{Discarded: discarded, From: from, Calls: opps}
	if err := np.send(proto.KindAskCall, body); err != nil {
		return game.Pass
	}
	env, err := np.recv()
	if err != nil {
		return game.Pass
	}
	if env.Kind != proto.KindAnswerCall {
		return game.Pass
	}
	var ans proto.AnswerCall
	if err := proto.DecodeBody(env, &ans); err != nil {
		return game.Pass
	}
	return ans.Call
}

func deref(t *tile.Tile) tile.Tile {
	if t == nil {
		return 0
	}
	return *t
}

// fallbackDiscard returns the first concealed tile when network IO failed.
// This keeps the game from deadlocking; the round will likely be lost.
func fallbackDiscard(view game.PlayerView) game.DrawAction {
	for i := 0; i < tile.NumKinds; i++ {
		if view.OwnHand.Concealed[i] > 0 {
			return game.DrawAction{Kind: game.DrawDiscard, Discard: tile.Tile(i)}
		}
	}
	return game.DrawAction{Kind: game.DrawDiscard}
}

// multicastObserver pushes StateUpdate messages to every remote player.
type multicastObserver struct {
	players []*netPlayer
}

func newMulticastObserver(players []*netPlayer) *multicastObserver {
	return &multicastObserver{players: players}
}

func (o *multicastObserver) push(s *game.State, note string) {
	for _, np := range o.players {
		if np == nil {
			continue
		}
		body := snapshotFor(s, np.seat, note)
		if err := np.send(proto.KindStateUpdate, body); err != nil {
			if !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "closed") {
				np.log.Warn("state_update send failed", "err", err)
			}
		}
	}
}

func (o *multicastObserver) OnRoundStart(s *game.State) { o.push(s, "round start") }
func (o *multicastObserver) OnExchange3(s *game.State, picks [game.NumPlayers][3]tile.Tile, direction int) {
	o.push(s, fmt.Sprintf("exchange-three direction %d", direction))
}
func (o *multicastObserver) OnDingque(s *game.State, seat int, suit tile.Suit) {
	o.push(s, fmt.Sprintf("seat %d dingque %d", seat, suit))
}
func (o *multicastObserver) OnDraw(s *game.State, seat int, t tile.Tile) {
	if seat == 0 {
		// No-op extra notification; AskDraw includes JustDrew.
	}
	o.push(s, fmt.Sprintf("seat %d draws", seat))
}
func (o *multicastObserver) OnDiscard(s *game.State, seat int, t tile.Tile) {
	o.push(s, fmt.Sprintf("seat %d discards %s", seat, t))
}
func (o *multicastObserver) OnCall(s *game.State, kind game.CallKind, seat, from int, t tile.Tile) {
	o.push(s, fmt.Sprintf("seat %d %s on %s (from %d)", seat, kind, t, from))
}
func (o *multicastObserver) OnWin(s *game.State, w game.WinEvent) {
	o.push(s, fmt.Sprintf("seat %d wins (fan=%d)", w.Seat, w.Score.Fan))
}
func (o *multicastObserver) OnRoundEnd(s *game.State, r *game.RoundResult) {
	// RoundEnd is sent separately by Run() after RunRoundWithObserver returns.
}

// snapshotFor builds a view-cropped StateUpdate for the given seat.
func snapshotFor(s *game.State, seat int, note string) proto.StateUpdate {
	upd := proto.StateUpdate{
		Note:     note,
		WallLeft: s.Wall.Remaining(),
		Dealer:   s.Dealer,
		Turn:     s.TurnsTaken,
		OwnHand:  s.Players[seat].Hand.Concealed,
		OwnMelds: s.Players[seat].Hand.Melds,
		JustDrew: s.Players[seat].JustDrew,
	}
	for i := 0; i < game.NumPlayers; i++ {
		upd.Seats[i] = proto.SeatPublic{
			Dingque:  s.Players[i].Dingque,
			HandSize: s.Players[i].Hand.ConcealedCount(),
			Melds:    s.Players[i].Hand.Melds,
			Discards: append([]tile.Tile{}, s.Discards[i]...),
			Score:    s.Players[i].Score,
			HasWon:   s.Players[i].HasWon,
		}
	}
	return upd
}
