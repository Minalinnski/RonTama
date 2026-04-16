// Package proto defines the wire format for RonTama LAN play.
//
// Messages are JSON-encoded one-per-line over a plain TCP connection.
// Each line is an Envelope: {"kind":"...","body":<inner-json>}.
// The receiver dispatches on Kind and unmarshals Body into the matching
// concrete type.
//
// View-cropped state is sent from server to clients; clients only
// receive information they would legally see at the table (own hand,
// melds of all players, all rivers, scores; never the wall or other
// players' concealed tiles).
package proto

import (
	"encoding/json"

	"github.com/Minalinnski/RonTama/internal/game"
	"github.com/Minalinnski/RonTama/internal/tile"
)

// Envelope is the wrapper used for every wire message.
type Envelope struct {
	Kind string          `json:"kind"`
	Body json.RawMessage `json:"body,omitempty"`
}

// Message kinds (server -> client).
const (
	KindHello        = "hello"
	KindRoundStart   = "round_start"
	KindStateUpdate  = "state_update"
	KindAskExchange3 = "ask_exchange3"
	KindAskDingque   = "ask_dingque"
	KindAskDraw      = "ask_draw"
	KindAskCall      = "ask_call"
	KindRoundEnd     = "round_end"
	KindError        = "error"
)

// Message kinds (client -> server).
const (
	KindRegister        = "register" // sent right after Hello with the client's display name
	KindAnswerExchange3 = "answer_exchange3"
	KindAnswerDingque   = "answer_dingque"
	KindAnswerDraw      = "answer_draw"
	KindAnswerCall      = "answer_call"
)

// Register is the first message a client sends after receiving Hello.
// Carries the user's display name for showing in others' TUIs and for
// future session-resumption matching.
type Register struct {
	Name string `json:"name"`
}

// Hello is the first message a client receives upon joining.
type Hello struct {
	Seat int    `json:"seat"`
	Rule string `json:"rule"`
}

// RoundStart announces a new round about to begin.
type RoundStart struct {
	Dealer int `json:"dealer"`
}

// StateUpdate is a view-cropped snapshot pushed after each public event.
type StateUpdate struct {
	Note     string         `json:"note"`
	WallLeft int            `json:"wall_left"`
	Dealer   int            `json:"dealer"`
	Turn     int            `json:"turn"`
	Seats    [4]SeatPublic  `json:"seats"`
	OwnHand  [tile.NumKinds]int `json:"own_hand"`
	OwnMelds []tile.Meld    `json:"own_melds"`
	JustDrew *tile.Tile     `json:"just_drew,omitempty"`
}

// SeatPublic is the per-seat info every observer sees.
type SeatPublic struct {
	Name     string      `json:"name"`
	Dingque  tile.Suit   `json:"dingque"`
	HandSize int         `json:"hand_size"`
	Melds    []tile.Meld `json:"melds"`
	Discards []tile.Tile `json:"discards"`
	Score    int         `json:"score"`
	HasWon   bool        `json:"has_won"`
}

// AskExchange3 asks the client for 3 tiles of one suit to pass.
type AskExchange3 struct {
	OwnHand    [tile.NumKinds]int `json:"own_hand"`
	TimeoutSec int                `json:"timeout_sec,omitempty"`
}

// AnswerExchange3 is the client's exchange-three picks.
type AnswerExchange3 struct {
	Tiles [3]tile.Tile `json:"tiles"`
}

// AskDingque asks the client for their dingque suit choice.
type AskDingque struct {
	OwnHand    [tile.NumKinds]int `json:"own_hand"`
	TimeoutSec int                `json:"timeout_sec,omitempty"`
}

// AskDraw asks the client for their post-draw action.
type AskDraw struct {
	OwnHand    [tile.NumKinds]int `json:"own_hand"`
	JustDrew   tile.Tile          `json:"just_drew"`
	Dingque    tile.Suit          `json:"dingque"`
	TimeoutSec int                `json:"timeout_sec,omitempty"`
}

// AskCall asks the client whether to call a discard.
type AskCall struct {
	Discarded  tile.Tile   `json:"discarded"`
	From       int         `json:"from"`
	Calls      []game.Call `json:"calls"`
	TimeoutSec int         `json:"timeout_sec,omitempty"`
}

// AnswerDingque is the client's dingque choice.
type AnswerDingque struct {
	Suit tile.Suit `json:"suit"`
}

// AnswerDraw is the client's post-draw action.
type AnswerDraw struct {
	Action game.DrawAction `json:"action"`
}

// AnswerCall is the client's call decision (Pass or one of the offered calls).
type AnswerCall struct {
	Call game.Call `json:"call"`
}

// RoundEnd announces the round result.
type RoundEnd struct {
	Result *game.RoundResult `json:"result"`
}

// ErrorMsg conveys a server-side error to the client.
type ErrorMsg struct {
	Message string `json:"message"`
}

// Encode marshals body into an Envelope JSON line.
func Encode(kind string, body any) ([]byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{Kind: kind, Body: raw})
}

// DecodeBody unmarshals env.Body into the concrete type backing kind.
func DecodeBody(env Envelope, dst any) error {
	if len(env.Body) == 0 {
		return nil
	}
	return json.Unmarshal(env.Body, dst)
}
