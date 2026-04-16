package server

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/Minalinnski/RonTama/internal/ai/tsumogiri"
	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/net/proto"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// sessionPlayer is a game.Player implementation that tolerates the
// underlying TCP connection going away mid-round.
//
// It holds a (possibly nil) *netPlayer for the live connection plus a
// tsumogiri fallback. All game.Player calls route to the netPlayer
// when one is attached; if it's nil (disconnect never happened yet,
// or already dropped) OR if the netPlayer returns an IO error, the
// call falls through to the fallback and the netPlayer is marked
// detached so future calls skip it entirely.
//
// Reconnection (future): server matches a new client's Register name
// against session.displayName and re-attaches via Attach().
type sessionPlayer struct {
	mu          sync.Mutex
	seat        int
	displayName string       // the name the human registered with (used for reconnect match)
	np          *netPlayer   // live connection; nil = disconnected
	fallback    *tsumogiri.Bot
	log         *slog.Logger
}

func newSessionPlayer(seat int, log *slog.Logger) *sessionPlayer {
	return &sessionPlayer{
		seat:     seat,
		log:      log,
		fallback: tsumogiri.New("seat" + fmt.Sprint(seat)),
	}
}

// Attach wires a live netPlayer to this session. The caller should
// have already read the Register message so displayName is populated.
func (s *sessionPlayer) Attach(np *netPlayer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.np = np
	if np.userName != "" {
		s.displayName = np.userName
	}
	s.fallback = tsumogiri.New(s.displayName)
}

// Detach drops the live connection, switching the session to fallback.
// Called on any IO error reading from or writing to the netPlayer.
func (s *sessionPlayer) Detach(reason error) {
	s.mu.Lock()
	np := s.np
	s.np = nil
	s.mu.Unlock()
	if np != nil {
		_ = np.conn.Close()
		s.log.Warn("client detached, falling back to tsumogiri",
			"seat", s.seat, "name", s.displayName, "err", reason)
	}
}

// Name returns the display name, with "(掉线)" appended when detached.
func (s *sessionPlayer) Name() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.np != nil {
		if s.displayName != "" {
			return s.displayName
		}
		return fmt.Sprintf("seat%d", s.seat)
	}
	return s.fallback.Name()
}

// Broadcast sends a fire-and-forget message to the attached netPlayer.
// No-op if detached. Used by the multicast observer to push
// StateUpdate / RoundEnd etc.
func (s *sessionPlayer) Broadcast(kind string, body any) {
	s.route(func(np *netPlayer) error {
		return np.send(kind, body)
	})
}

// route runs fn against the netPlayer if attached; if the netPlayer
// surface-returns an error, detaches and returns false so the caller
// falls back to the tsumogiri bot.
func (s *sessionPlayer) route(fn func(*netPlayer) error) bool {
	s.mu.Lock()
	np := s.np
	s.mu.Unlock()
	if np == nil {
		return false
	}
	if err := fn(np); err != nil {
		if isDisconnectErr(err) {
			s.Detach(err)
			return false
		}
		// Non-disconnect error: log but keep the session live.
		s.log.Warn("netPlayer call error", "seat", s.seat, "err", err)
		return false
	}
	return true
}

// isDisconnectErr heuristically detects socket-dead situations.
func isDisconnectErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "closed") ||
		strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "deadline exceeded") ||
		strings.Contains(s, "i/o timeout")
}

// ---- game.Player implementation ----

func (s *sessionPlayer) ChooseExchange3(view game.PlayerView) [3]tile.Tile {
	var result [3]tile.Tile
	var got bool
	s.route(func(np *netPlayer) error {
		body := proto.AskExchange3{OwnHand: view.OwnHand.Concealed, TimeoutSec: np.timeoutSec()}
		if err := np.send(proto.KindAskExchange3, body); err != nil {
			return err
		}
		env, err := np.recv()
		if err != nil {
			return err
		}
		if env.Kind != proto.KindAnswerExchange3 {
			return fmt.Errorf("expected answer_exchange3, got %q", env.Kind)
		}
		var ans proto.AnswerExchange3
		if err := proto.DecodeBody(env, &ans); err != nil {
			return err
		}
		result = ans.Tiles
		got = true
		return nil
	})
	if got {
		return result
	}
	return s.fallback.ChooseExchange3(view)
}

func (s *sessionPlayer) ChooseDingque(view game.PlayerView) tile.Suit {
	var result tile.Suit
	var got bool
	s.route(func(np *netPlayer) error {
		body := proto.AskDingque{OwnHand: view.OwnHand.Concealed, TimeoutSec: np.timeoutSec()}
		if err := np.send(proto.KindAskDingque, body); err != nil {
			return err
		}
		env, err := np.recv()
		if err != nil {
			return err
		}
		if env.Kind != proto.KindAnswerDingque {
			return fmt.Errorf("expected answer_dingque, got %q", env.Kind)
		}
		var ans proto.AnswerDingque
		if err := proto.DecodeBody(env, &ans); err != nil {
			return err
		}
		result = ans.Suit
		got = true
		return nil
	})
	if got {
		return result
	}
	return s.fallback.ChooseDingque(view)
}

func (s *sessionPlayer) OnDraw(view game.PlayerView) game.DrawAction {
	var result game.DrawAction
	var got bool
	s.route(func(np *netPlayer) error {
		body := proto.AskDraw{
			OwnHand:    view.OwnHand.Concealed,
			JustDrew:   derefTile(view.JustDrew),
			Dingque:    view.Dingque[view.Seat],
			TimeoutSec: np.timeoutSec(),
		}
		if err := np.send(proto.KindAskDraw, body); err != nil {
			return err
		}
		env, err := np.recv()
		if err != nil {
			return err
		}
		if env.Kind != proto.KindAnswerDraw {
			return fmt.Errorf("expected answer_draw, got %q", env.Kind)
		}
		var ans proto.AnswerDraw
		if err := proto.DecodeBody(env, &ans); err != nil {
			return err
		}
		result = ans.Action
		got = true
		return nil
	})
	if got {
		return result
	}
	return s.fallback.OnDraw(view)
}

func (s *sessionPlayer) OnCallOpportunity(view game.PlayerView, discarded tile.Tile, from int, opps []game.Call) game.Call {
	var result game.Call
	var got bool
	s.route(func(np *netPlayer) error {
		body := proto.AskCall{Discarded: discarded, From: from, Calls: opps, TimeoutSec: np.timeoutSec()}
		if err := np.send(proto.KindAskCall, body); err != nil {
			return err
		}
		env, err := np.recv()
		if err != nil {
			return err
		}
		if env.Kind != proto.KindAnswerCall {
			return fmt.Errorf("expected answer_call, got %q", env.Kind)
		}
		var ans proto.AnswerCall
		if err := proto.DecodeBody(env, &ans); err != nil {
			return err
		}
		result = ans.Call
		got = true
		return nil
	})
	if got {
		return result
	}
	return s.fallback.OnCallOpportunity(view, discarded, from, opps)
}

func derefTile(t *tile.Tile) tile.Tile {
	if t == nil {
		return 0
	}
	return *t
}
