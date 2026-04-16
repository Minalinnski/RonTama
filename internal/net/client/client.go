// Package client connects to a RonTama LAN server and translates the
// wire protocol into game.Player decisions backed by a Decider.
//
// The Decider is what makes the same client work in both headless
// (bot-driven) and interactive (TUI-driven) modes:
//   - HeadlessDecider wraps an existing game.Player (e.g. easy.Bot)
//     and answers requests automatically; useful for testing and bot-only matches
//   - TUIDecider (in cmd/rontama) prompts a human via Bubble Tea
package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/net/proto"
	"github.com/Minalinnski/RonTama/internal/rules"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// Decider is the policy that turns server-side prompts into client answers.
type Decider interface {
	// AssignSeat is called once after Hello arrives. The client knows its seat.
	AssignSeat(seat int, ruleName string)

	// HandleStateUpdate is called for every StateUpdate the client receives.
	HandleStateUpdate(upd proto.StateUpdate)

	// AnswerExchange3/AnswerDingque/AnswerDraw/AnswerCall produce responses for prompts.
	AnswerExchange3(req proto.AskExchange3) [3]tile.Tile
	AnswerDingque(req proto.AskDingque) tile.Suit
	AnswerDraw(req proto.AskDraw) game.DrawAction
	AnswerCall(req proto.AskCall) game.Call

	// HandleRoundEnd / HandleError are notified terminally.
	HandleRoundEnd(end proto.RoundEnd)
	HandleError(msg proto.ErrorMsg)
}

// Client wraps a TCP connection to a server.
type Client struct {
	conn    net.Conn
	enc     *bufio.Writer
	dec     *json.Decoder
	decider Decider
	log     *slog.Logger
	mu      sync.Mutex
}

// Dial connects to addr and returns a Client.
func Dial(addr string, decider Decider, log *slog.Logger) (*Client, error) {
	return DialAs(addr, "", decider, log)
}

// DialAs connects to addr and registers with the given display name.
// Empty name → server uses a synthetic 'net-N'.
func DialAs(addr, name string, decider Decider, log *slog.Logger) (*Client, error) {
	if log == nil {
		log = slog.Default()
	}
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	cli := &Client{
		conn:    c,
		enc:     bufio.NewWriter(c),
		dec:     json.NewDecoder(bufio.NewReader(c)),
		decider: decider,
		log:     log,
	}
	if name != "" {
		// Registration is fire-and-forget; the server reads it right after Hello.
		if err := cli.send(proto.KindRegister, proto.Register{Name: name}); err != nil {
			log.Warn("register send failed", "err", err)
		}
	}
	return cli, nil
}

// Close terminates the connection.
func (c *Client) Close() error { return c.conn.Close() }

// Run reads and dispatches messages until EOF or RoundEnd.
func (c *Client) Run() error {
	for {
		var env proto.Envelope
		if err := c.dec.Decode(&env); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("recv: %w", err)
		}
		if err := c.dispatch(env); err != nil {
			return err
		}
		// Treat RoundEnd as terminal.
		if env.Kind == proto.KindRoundEnd {
			return nil
		}
	}
}

func (c *Client) dispatch(env proto.Envelope) error {
	switch env.Kind {
	case proto.KindHello:
		var h proto.Hello
		if err := proto.DecodeBody(env, &h); err != nil {
			return err
		}
		c.decider.AssignSeat(h.Seat, h.Rule)
		c.log.Info("hello received", "seat", h.Seat, "rule", h.Rule)
	case proto.KindStateUpdate:
		var u proto.StateUpdate
		if err := proto.DecodeBody(env, &u); err != nil {
			return err
		}
		c.decider.HandleStateUpdate(u)
	case proto.KindAskExchange3:
		var r proto.AskExchange3
		if err := proto.DecodeBody(env, &r); err != nil {
			return err
		}
		ans := c.decider.AnswerExchange3(r)
		return c.send(proto.KindAnswerExchange3, proto.AnswerExchange3{Tiles: ans})
	case proto.KindAskDingque:
		var r proto.AskDingque
		if err := proto.DecodeBody(env, &r); err != nil {
			return err
		}
		ans := c.decider.AnswerDingque(r)
		return c.send(proto.KindAnswerDingque, proto.AnswerDingque{Suit: ans})
	case proto.KindAskDraw:
		var r proto.AskDraw
		if err := proto.DecodeBody(env, &r); err != nil {
			return err
		}
		ans := c.decider.AnswerDraw(r)
		return c.send(proto.KindAnswerDraw, proto.AnswerDraw{Action: ans})
	case proto.KindAskCall:
		var r proto.AskCall
		if err := proto.DecodeBody(env, &r); err != nil {
			return err
		}
		ans := c.decider.AnswerCall(r)
		return c.send(proto.KindAnswerCall, proto.AnswerCall{Call: ans})
	case proto.KindRoundEnd:
		var r proto.RoundEnd
		if err := proto.DecodeBody(env, &r); err != nil {
			return err
		}
		c.decider.HandleRoundEnd(r)
	case proto.KindError:
		var r proto.ErrorMsg
		_ = proto.DecodeBody(env, &r)
		c.decider.HandleError(r)
	default:
		c.log.Warn("unknown message kind", "kind", env.Kind)
	}
	return nil
}

func (c *Client) send(kind string, body any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	line, err := proto.Encode(kind, body)
	if err != nil {
		return err
	}
	if _, err := c.enc.Write(line); err != nil {
		return err
	}
	if err := c.enc.WriteByte('\n'); err != nil {
		return err
	}
	return c.enc.Flush()
}

// HeadlessDecider wraps a game.Player so a bot can play over the network.
//
// It reconstructs a synthesized PlayerView from each prompt's body. This
// lets us run a full client-side bot without ever building one server-side.
type HeadlessDecider struct {
	bot       game.Player
	rule      rules.RuleSet
	seat      int
	dingque   [game.NumPlayers]tile.Suit
	hasWon    [game.NumPlayers]bool
	melds     [game.NumPlayers][]tile.Meld
	discards  [game.NumPlayers][]tile.Tile
	scores    [game.NumPlayers]int
	dealer    int
	wallLeft  int
	turn      int
}

// NewHeadlessDecider wraps bot with the given rule.
func NewHeadlessDecider(bot game.Player, rule rules.RuleSet) *HeadlessDecider {
	d := &HeadlessDecider{bot: bot, rule: rule}
	for i := 0; i < game.NumPlayers; i++ {
		d.dingque[i] = tile.SuitWind
	}
	return d
}

// AssignSeat implements Decider.
func (d *HeadlessDecider) AssignSeat(seat int, _ string) { d.seat = seat }

// AnswerExchange3 implements Decider.
func (d *HeadlessDecider) AnswerExchange3(req proto.AskExchange3) [3]tile.Tile {
	view := game.PlayerView{
		Rule:    d.rule,
		Seat:    d.seat,
		OwnHand: tile.Hand{Concealed: req.OwnHand},
	}
	return d.bot.ChooseExchange3(view)
}

// HandleStateUpdate implements Decider.
func (d *HeadlessDecider) HandleStateUpdate(upd proto.StateUpdate) {
	d.dealer = upd.Dealer
	d.wallLeft = upd.WallLeft
	d.turn = upd.Turn
	for i := 0; i < game.NumPlayers; i++ {
		d.dingque[i] = upd.Seats[i].Dingque
		d.hasWon[i] = upd.Seats[i].HasWon
		d.melds[i] = upd.Seats[i].Melds
		d.discards[i] = upd.Seats[i].Discards
		d.scores[i] = upd.Seats[i].Score
	}
}

// AnswerDingque implements Decider.
func (d *HeadlessDecider) AnswerDingque(req proto.AskDingque) tile.Suit {
	view := game.PlayerView{
		Rule:    d.rule,
		Seat:    d.seat,
		Dealer:  d.dealer,
		OwnHand: tile.Hand{Concealed: req.OwnHand},
	}
	return d.bot.ChooseDingque(view)
}

// AnswerDraw implements Decider.
func (d *HeadlessDecider) AnswerDraw(req proto.AskDraw) game.DrawAction {
	jd := req.JustDrew
	view := game.PlayerView{
		Rule:     d.rule,
		Seat:     d.seat,
		Dealer:   d.dealer,
		WallLeft: d.wallLeft,
		OwnHand:  tile.Hand{Concealed: req.OwnHand},
		JustDrew: &jd,
		Dingque:  d.dingque,
		HasWon:   d.hasWon,
		Discards: d.discards,
		Melds:    d.melds,
		Scores:   d.scores,
	}
	return d.bot.OnDraw(view)
}

// AnswerCall implements Decider.
func (d *HeadlessDecider) AnswerCall(req proto.AskCall) game.Call {
	view := game.PlayerView{
		Rule:     d.rule,
		Seat:     d.seat,
		Dealer:   d.dealer,
		Dingque:  d.dingque,
		HasWon:   d.hasWon,
		Discards: d.discards,
		Melds:    d.melds,
		Scores:   d.scores,
	}
	return d.bot.OnCallOpportunity(view, req.Discarded, req.From, req.Calls)
}

// HandleRoundEnd implements Decider.
func (d *HeadlessDecider) HandleRoundEnd(end proto.RoundEnd) {}

// HandleError implements Decider.
func (d *HeadlessDecider) HandleError(msg proto.ErrorMsg) {}
