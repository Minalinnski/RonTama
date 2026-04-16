// Package tui (cont.) — unified pre-game room view.
//
// The GameRoom is a structured model that replaces the old ad-hoc
// Banner string. All three play modes (local, host, join) create a
// RoomState with their own seat configuration; the rendering is
// identical. Differences are captured by flags (CanStart, ShowIPs).
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Minalinnski/RonTama/internal/discovery"
)

// SeatStatus classifies what a seat shows in the room view.
type SeatStatus int

const (
	SeatStatusYou      SeatStatus = iota // local human player
	SeatStatusBot                        // local bot (with tier name)
	SeatStatusWaiting                    // remote slot, nobody connected yet
	SeatStatusJoined                     // remote player connected
	SeatStatusOffline                    // was connected, now disconnected
)

// SeatInfo describes one seat in the room.
type SeatInfo struct {
	Name   string     // display name ("alice", "Easy bot 1", "")
	Status SeatStatus // visual cue
}

// RoomState is the pre-game lobby state. Set on PlayModel before
// NewProgram (for local/host) or updated live via RoomUpdateMsg
// (from server JoinEvents or client StateUpdates).
type RoomState struct {
	Seats    [4]SeatInfo
	Rule     string // "sichuan" / "riichi"
	CanStart bool   // true = host (show "s = start")
	ShowIPs  bool   // true = host (show local IP addresses)
	Message  string // bottom status ("Waiting for host..." / "Press s to start" / etc.)

	// Filled/Total for progress bar. Filled = non-waiting seats.
	Filled int
	Total  int // total "expected" seats (remote slots)
}

// RoomUpdateMsg updates the room state during the join phase.
// Sent by either the JoinEvent-pump goroutine (host) or the client
// TUIDecider (join). PlayModel.Update applies it to Room.
type RoomUpdateMsg struct {
	Room RoomState
}

// renderRoom renders the unified room view.
func renderRoom(room *RoomState, startSent bool) string {
	if room == nil {
		return "Connecting to game..."
	}
	var b strings.Builder

	// Title
	title := lipgloss.NewStyle().Bold(true).Foreground(headerColor).
		Render(fmt.Sprintf("🀄  Game Room — %s", room.Rule))
	b.WriteString(title)
	b.WriteString("\n\n")

	// Seats
	for i, seat := range room.Seats {
		var icon, nameStr string
		nameStyle := lipgloss.NewStyle()
		switch seat.Status {
		case SeatStatusYou:
			icon = "✓"
			nameStr = seat.Name + " (You)"
			nameStyle = nameStyle.Foreground(lipgloss.Color("#3FB76C")).Bold(true)
		case SeatStatusJoined:
			icon = "✓"
			nameStr = seat.Name
			nameStyle = nameStyle.Foreground(lipgloss.Color("#3FB76C"))
		case SeatStatusBot:
			icon = "🤖"
			nameStr = seat.Name
			nameStyle = nameStyle.Foreground(chromeColor)
		case SeatStatusWaiting:
			icon = "⏳"
			nameStr = "waiting..."
			nameStyle = nameStyle.Foreground(dimColor)
		case SeatStatusOffline:
			icon = "✗"
			nameStr = seat.Name + " (offline)"
			nameStyle = nameStyle.Foreground(lipgloss.Color("#E45757"))
		}
		label := fmt.Sprintf("  Seat %d (%s):  %s %s", i, seatLabel(i), icon, nameStyle.Render(nameStr))
		b.WriteString(label)
		b.WriteString("\n")
	}

	// Progress bar
	if room.Total > 0 {
		pct := float64(room.Filled) / float64(room.Total)
		filled := int(pct * 20)
		bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
		b.WriteString(fmt.Sprintf("\n  %s  %d/%d players\n", bar, room.Filled, room.Total))
	}

	// IPs (host only)
	if room.ShowIPs {
		addrs := discovery.LocalIPv4Addrs()
		if len(addrs) > 0 {
			b.WriteString("\n")
			b.WriteString(chromeStyle.Render("  朋友加入: Join by IP →"))
			for _, a := range addrs {
				b.WriteString(fmt.Sprintf(" %s:7777", a))
			}
			b.WriteString("\n")
		}
	}

	// Controls / status
	b.WriteString("\n")
	if room.CanStart && !startSent {
		b.WriteString(promptStyle.Render("  s = start (空位填bot)  ·  q = quit"))
	} else if room.CanStart && startSent {
		b.WriteString(promptStyle.Render("  Starting..."))
	} else {
		// Client view
		msg := room.Message
		if msg == "" {
			msg = "Waiting for host to start..."
		}
		b.WriteString(chromeStyle.Render("  " + msg + "  ·  q = quit"))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(headerColor).
		Padding(1, 2).
		Render(b.String())
}
