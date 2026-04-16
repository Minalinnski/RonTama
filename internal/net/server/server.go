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
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

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

	// Rule overrides the default (sichuan) when set.
	Rule rules.RuleSet

	// Players: per-seat pre-assigned game.Player. nil entries become
	// "remote slots" that the server fills from incoming connections.
	// After JoinTimeout, any still-empty remote slots default to easy.Bot.
	//
	// Nil-filled Players (the zero value) is treated as "all remote" —
	// useful for legacy callers / tests.
	Players [game.NumPlayers]game.Player

	// ExtraObserver is invoked alongside the always-present multicast
	// observer. Lets a TUI host see local state changes too.
	ExtraObserver game.Observer

	// PromptTimeout: how long the server waits for a remote client's
	// answer to any Ask* message. On expiry the server applies the
	// fallback default (Pass for call, first concealed tile for
	// discard, SuitMan for dingque). Default: 30 seconds.
	PromptTimeout time.Duration

	// JoinChan, when non-nil, receives live updates during the join
	// phase so the host's TUI can show a real-time lobby with countdown,
	// seat status, and player names. Closed by Server.Run when the
	// join phase ends (game starting or timeout).
	JoinChan chan<- JoinEvent
}

// JoinEvent describes a join-phase state change pushed to JoinChan.
type JoinEvent struct {
	// Seats: per-seat status. "" = waiting, non-empty = joined (value = name).
	Seats    [game.NumPlayers]string
	Filled   int           // how many remote seats have been filled
	Total    int           // total remote seats expected
	TimeLeft time.Duration // time until unfilled seats become bots
	Done     bool          // true = join phase over, game starting
}

// Run starts the server and blocks until the round finishes (or ctx is cancelled).
//
// Seat plan: cfg.Players seats with non-nil values are used as-is
// (local human via TUI, local bot, etc.). Nil entries are remote slots
// that get filled by incoming TCP connections; after JoinTimeout, any
// still-nil seats fall back to easy.Bot.
func Run(ctx context.Context, cfg Config) error {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	rule := cfg.Rule
	if rule == nil {
		rule = sichuan.New()
	}
	promptTimeout := cfg.PromptTimeout
	if promptTimeout <= 0 {
		promptTimeout = 30 * time.Second
	}

	// Identify remote slots (nil entries in cfg.Players).
	remoteSeats := []int{}
	for i, p := range cfg.Players {
		if p == nil {
			remoteSeats = append(remoteSeats, i)
		}
	}

	var ln net.Listener
	if len(remoteSeats) > 0 {
		lc := &net.ListenConfig{}
		var err error
		ln, err = lc.Listen(ctx, "tcp", cfg.Addr)
		if err != nil {
			return fmt.Errorf("listen %s: %w", cfg.Addr, err)
		}
		defer ln.Close()
		cfg.Log.Info("server listening", "addr", ln.Addr(), "remote_seats", remoteSeats)
	} else {
		cfg.Log.Info("no remote seats — running pure local game")
	}

	connCh := make(chan net.Conn, game.NumPlayers)
	if ln != nil {
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				select {
				case connCh <- c:
				default:
					_ = c.Close()
				}
			}
		}()
	}

	timeout := cfg.JoinTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	startTime := time.Now()
	var deadline <-chan time.Time
	if len(remoteSeats) > 0 {
		deadline = time.After(timeout)
		cfg.Log.Info("waiting for joiners", "timeout", timeout, "needed", len(remoteSeats))
	}

	players := cfg.Players
	sessions := map[int]*sessionPlayer{}
	for _, seat := range remoteSeats {
		sess := newSessionPlayer(seat, cfg.Log)
		sessions[seat] = sess
		players[seat] = sess
	}
	connected := 0

	// Helper: push a JoinEvent snapshot to the host TUI (if channel provided).
	pushJoinEvent := func(done bool) {
		if cfg.JoinChan == nil {
			return
		}
		var seats [game.NumPlayers]string
		for i := 0; i < game.NumPlayers; i++ {
			if p := cfg.Players[i]; p != nil {
				seats[i] = p.Name()
			}
			if sess, ok := sessions[i]; ok {
				if sess.displayName != "" {
					seats[i] = sess.displayName
				} else {
					seats[i] = "" // still waiting
				}
			}
		}
		elapsed := time.Since(startTime)
		left := timeout - elapsed
		if left < 0 {
			left = 0
		}
		cfg.JoinChan <- JoinEvent{
			Seats:    seats,
			Filled:   connected,
			Total:    len(remoteSeats),
			TimeLeft: left,
			Done:     done,
		}
	}

	// Tick every second to update the countdown in the host TUI.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	pushJoinEvent(false)

awaitLoop:
	for connected < len(remoteSeats) {
		select {
		case c := <-connCh:
			seat := remoteSeats[connected]
			np := newNetPlayer(c, seat, rule, cfg.Log)
			np.timeout = promptTimeout
			if err := np.sendHello(); err != nil {
				cfg.Log.Warn("hello send failed", "seat", seat, "err", err)
				_ = c.Close()
				continue
			}
			np.readRegister()
			sessions[seat].Attach(np)
			connected++
			cfg.Log.Info("client joined", "seat", seat, "name", np.userName, "remote", c.RemoteAddr())
			pushJoinEvent(false)
		case <-ticker.C:
			pushJoinEvent(false)
		case <-deadline:
			cfg.Log.Info("join timeout — unconnected remote seats default to tsumogiri bot",
				"attached", connected, "pending", len(remoteSeats)-connected)
			break awaitLoop
		case <-ctx.Done():
			if cfg.JoinChan != nil {
				close(cfg.JoinChan)
			}
			return ctx.Err()
		}
	}
	pushJoinEvent(true)
	if cfg.JoinChan != nil {
		close(cfg.JoinChan)
	}

	// Start a reconnection-listener goroutine that drains connCh for the
	// rest of the round: any new connection whose Register name matches
	// a currently-detached session re-attaches to that session.
	reconCtx, reconCancel := context.WithCancel(ctx)
	defer reconCancel()
	go reconnectLoop(reconCtx, connCh, sessions, promptTimeout, rule, cfg.Log)

	mc := newMulticastObserver(sessions)
	var obs game.Observer = mc
	if cfg.ExtraObserver != nil {
		obs = chainObservers(mc, cfg.ExtraObserver)
	}

	res, err := game.RunRoundWithObserver(rule, players, 0, cfg.Log, obs)
	reconCancel()

	for _, sess := range sessions {
		sess.Broadcast(proto.KindRoundEnd, proto.RoundEnd{Result: res})
		sess.Detach(nil)
	}
	return err
}

// reconnectLoop handles connections that arrive after the initial seat
// allocation. A new client sends Register; if the name matches a
// detached session, reattach; otherwise reject.
func reconnectLoop(ctx context.Context, connCh <-chan net.Conn, sessions map[int]*sessionPlayer, timeout time.Duration, rule rules.RuleSet, log *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-connCh:
			go handleReconnect(c, sessions, timeout, rule, log)
		}
	}
}

func handleReconnect(c net.Conn, sessions map[int]*sessionPlayer, timeout time.Duration, rule rules.RuleSet, log *slog.Logger) {
	// Assign a placeholder seat (-1) temporarily; the real seat comes
	// from whichever session we reattach to.
	np := newNetPlayer(c, -1, rule, log)
	np.timeout = timeout
	// Send a generic Hello with seat=-1; the client will know the real
	// seat once reattached (it matches on name).
	if err := np.send(proto.KindHello, proto.Hello{Seat: -1, Rule: rule.Name()}); err != nil {
		_ = c.Close()
		return
	}
	np.readRegister()
	if np.userName == "" {
		log.Warn("reconnect attempt without name; rejecting", "remote", c.RemoteAddr())
		_ = np.send(proto.KindError, proto.ErrorMsg{Message: "name required for reconnect"})
		_ = c.Close()
		return
	}
	for seat, sess := range sessions {
		sess.mu.Lock()
		alreadyAttached := sess.np != nil
		nameMatch := sess.displayName == np.userName
		sess.mu.Unlock()
		if !alreadyAttached && nameMatch {
			np.seat = seat
			// Send a fresh Hello with the correct seat so the client
			// can lay out the TUI from its real position.
			_ = np.send(proto.KindHello, proto.Hello{Seat: seat, Rule: rule.Name()})
			sess.Attach(np)
			log.Info("client reconnected", "seat", seat, "name", np.userName)
			return
		}
	}
	log.Warn("reconnect rejected — no matching detached session", "name", np.userName)
	_ = np.send(proto.KindError, proto.ErrorMsg{Message: "no seat waiting for this name"})
	_ = c.Close()
}

// chainObservers composes two observers into one (both fire on every event).
func chainObservers(a, b game.Observer) game.Observer { return &chainObs{a: a, b: b} }

type chainObs struct{ a, b game.Observer }

func (c *chainObs) OnRoundStart(s *game.State) { c.a.OnRoundStart(s); c.b.OnRoundStart(s) }
func (c *chainObs) OnExchange3(s *game.State, p [game.NumPlayers][3]tile.Tile, d int) {
	c.a.OnExchange3(s, p, d)
	c.b.OnExchange3(s, p, d)
}
func (c *chainObs) OnDingque(s *game.State, seat int, suit tile.Suit) {
	c.a.OnDingque(s, seat, suit)
	c.b.OnDingque(s, seat, suit)
}
func (c *chainObs) OnDraw(s *game.State, seat int, t tile.Tile) {
	c.a.OnDraw(s, seat, t)
	c.b.OnDraw(s, seat, t)
}
func (c *chainObs) OnDiscard(s *game.State, seat int, t tile.Tile) {
	c.a.OnDiscard(s, seat, t)
	c.b.OnDiscard(s, seat, t)
}
func (c *chainObs) OnCall(s *game.State, k game.CallKind, seat, from int, t tile.Tile) {
	c.a.OnCall(s, k, seat, from, t)
	c.b.OnCall(s, k, seat, from, t)
}
func (c *chainObs) OnWin(s *game.State, w game.WinEvent) { c.a.OnWin(s, w); c.b.OnWin(s, w) }
func (c *chainObs) OnRoundEnd(s *game.State, r *game.RoundResult) {
	c.a.OnRoundEnd(s, r)
	c.b.OnRoundEnd(s, r)
}

// netPlayer is a game.Player backed by a TCP connection.
type netPlayer struct {
	conn     net.Conn
	seat     int
	rule     rules.RuleSet
	log      *slog.Logger
	timeout  time.Duration // per-prompt read deadline; 0 = wait forever
	userName string         // populated from Register message

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

func (np *netPlayer) Name() string {
	if np.userName != "" {
		return np.userName
	}
	return fmt.Sprintf("net-%d", np.seat)
}

// readRegister blocks briefly waiting for the client's KindRegister
// message right after Hello. Best-effort: a misbehaving client can
// skip it and we just fall back to the synthetic 'net-N' name.
func (np *netPlayer) readRegister() {
	np.mu.Lock()
	defer np.mu.Unlock()
	_ = np.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	defer np.conn.SetReadDeadline(time.Time{})
	var env proto.Envelope
	if err := np.dec.Decode(&env); err != nil {
		return
	}
	if env.Kind != proto.KindRegister {
		return
	}
	var reg proto.Register
	if err := proto.DecodeBody(env, &reg); err != nil {
		return
	}
	np.userName = reg.Name
}

// ChooseExchange3 implements game.Player by relaying to the client.
func (np *netPlayer) ChooseExchange3(view game.PlayerView) [3]tile.Tile {
	body := proto.AskExchange3{OwnHand: view.OwnHand.Concealed, TimeoutSec: np.timeoutSec()}
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
	if np.timeout > 0 {
		_ = np.conn.SetReadDeadline(time.Now().Add(np.timeout))
		defer np.conn.SetReadDeadline(time.Time{})
	}
	var env proto.Envelope
	if err := np.dec.Decode(&env); err != nil {
		return env, err
	}
	return env, nil
}

// timeoutSec returns the prompt timeout in whole seconds for inclusion
// in Ask* messages so the client can render a countdown.
func (np *netPlayer) timeoutSec() int {
	if np.timeout <= 0 {
		return 0
	}
	return int(np.timeout / time.Second)
}

func (np *netPlayer) ChooseDingque(view game.PlayerView) tile.Suit {
	body := proto.AskDingque{OwnHand: view.OwnHand.Concealed, TimeoutSec: np.timeoutSec()}
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
		OwnHand:    view.OwnHand.Concealed,
		JustDrew:   deref(view.JustDrew),
		Dingque:    view.Dingque[view.Seat],
		TimeoutSec: np.timeoutSec(),
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
	body := proto.AskCall{Discarded: discarded, From: from, Calls: opps, TimeoutSec: np.timeoutSec()}
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

// multicastObserver pushes StateUpdate messages to every session's
// attached netPlayer. Detached sessions simply no-op.
type multicastObserver struct {
	sessions map[int]*sessionPlayer
}

func newMulticastObserver(sessions map[int]*sessionPlayer) *multicastObserver {
	return &multicastObserver{sessions: sessions}
}

func (o *multicastObserver) push(s *game.State, note string) {
	for seat, sess := range o.sessions {
		body := snapshotFor(s, seat, note)
		sess.Broadcast(proto.KindStateUpdate, body)
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
			Name:     s.Players[i].Name,
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
