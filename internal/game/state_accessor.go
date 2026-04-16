package game

import "github.com/Minalinnski/RonTama/internal/tile"

// StateAccessor implementation on *State so hooks (which can't import
// game due to the rules→game cycle) can read/write state through the
// rules.StateAccessor interface.

// StateAccessor method names use Get* prefix to avoid colliding with
// the exported fields of the same name on State (Go forbids a struct
// from having both a field and a method with the same name).
func (s *State) GetNumPlayers() int                       { return NumPlayers }
func (s *State) GetDealer() int                           { return s.Dealer }
func (s *State) GetCurrent() int                          { return s.Current }
func (s *State) GetTurnsTaken() int                       { return s.TurnsTaken }
func (s *State) GetWallRemaining() int                    { return s.Wall.Remaining() }
func (s *State) GetPlayerConcealed(seat int) [tile.NumKinds]int { return s.Players[seat].Hand.Concealed }
func (s *State) GetPlayerMelds(seat int) []tile.Meld      { return s.Players[seat].Hand.Melds }
func (s *State) GetPlayerScore(seat int) int              { return s.Players[seat].Score }
func (s *State) GetPlayerDingque(seat int) tile.Suit      { return s.Players[seat].Dingque }
func (s *State) GetPlayerJustDrew(seat int) *tile.Tile    { return s.Players[seat].JustDrew }
func (s *State) GetPlayerHasWon(seat int) bool            { return s.Players[seat].HasWon }
func (s *State) GetPlayerName(seat int) string            { return s.Players[seat].Name }
func (s *State) GetDiscards(seat int) []tile.Tile         { return s.Discards[seat] }
func (s *State) GetAllowsChi() bool                       { return s.Rule.AllowsChi() }
func (s *State) GetAfterKan() bool                        { return s.AfterKan }
func (s *State) GetLastTile() bool                        { return s.Wall.Remaining() == 0 }
func (s *State) SetPlayerScore(seat, score int)           { s.Players[seat].Score = score }
func (s *State) SetAfterKan(v bool)                       { s.AfterKan = v }
func (s *State) SetRuleState(v any)                       { s.RuleState = v }
func (s *State) GetRuleState() any                        { return s.RuleState }
func (s *State) DrawFromWallBack(n int) []tile.Tile       { return s.Wall.SplitDeadWall(n) }
func (s *State) DrawFromWall() (tile.Tile, bool)          { return s.Wall.Draw() }
