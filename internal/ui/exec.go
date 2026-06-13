package ui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

// termInput is one decoded keystroke destined for the emulator. Either text is
// sent verbatim or a key event is sent for the emulator to encode.
type termInput struct {
	text  string
	key   vt.KeyPressEvent
	isKey bool
}

// specialKeys maps Bubble Tea key types to virtual-terminal key codes so the
// emulator can encode them per the active terminal modes (e.g. application
// cursor keys), which raw byte injection would get wrong.
var specialKeys = map[tea.KeyType]rune{
	tea.KeyEnter:     uv.KeyEnter,
	tea.KeyTab:       uv.KeyTab,
	tea.KeyBackspace: uv.KeyBackspace,
	tea.KeyEsc:       uv.KeyEscape,
	tea.KeyDelete:    uv.KeyDelete,
	tea.KeyUp:        uv.KeyUp,
	tea.KeyDown:      uv.KeyDown,
	tea.KeyLeft:      uv.KeyLeft,
	tea.KeyRight:     uv.KeyRight,
	tea.KeyHome:      uv.KeyHome,
	tea.KeyEnd:       uv.KeyEnd,
	tea.KeyPgUp:      uv.KeyPgUp,
	tea.KeyPgDown:    uv.KeyPgDown,
}

// translateKey converts a Bubble Tea key event into emulator input. Bubble Tea
// has already parsed stdin into key events, so we re-encode rather than
// forwarding raw bytes. Returns false for keys with no mapping.
func translateKey(msg tea.KeyMsg) (termInput, bool) {
	// Alt-modified printable key (meta).
	if msg.Alt && msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		return termInput{isKey: true, key: vt.KeyPressEvent{Code: msg.Runes[0], Mod: vt.ModAlt}}, true
	}

	switch msg.Type {
	case tea.KeyRunes:
		return termInput{text: string(msg.Runes)}, true
	case tea.KeySpace:
		return termInput{text: " "}, true
	}

	if code, ok := specialKeys[msg.Type]; ok {
		ev := vt.KeyPressEvent{Code: code}
		if msg.Alt {
			ev.Mod |= vt.ModAlt
		}
		return termInput{isKey: true, key: ev}, true
	}

	// Ctrl + single char (covers ctrl+c, ctrl+d, ctrl+z, etc.).
	if s := msg.String(); strings.HasPrefix(s, "ctrl+") {
		rest := strings.TrimPrefix(s, "ctrl+")
		if len(rest) == 1 {
			return termInput{isKey: true, key: vt.KeyPressEvent{Code: rune(rest[0]), Mod: vt.ModCtrl}}, true
		}
	}
	return termInput{}, false
}

// runTermInput applies queued keystrokes to the emulator on its own goroutine.
// SendText/SendKey write to a pipe that blocks until the exec stdin reader
// drains it, so keeping this off the UI loop prevents the TUI from freezing if
// the stream stalls. It exits when ctx is cancelled (the pending pipe write is
// unblocked by the emulator being closed during teardown).
func runTermInput(ctx context.Context, em *vt.SafeEmulator, ch <-chan termInput) {
	for {
		select {
		case <-ctx.Done():
			return
		case in := <-ch:
			if in.isKey {
				em.SendKey(in.key)
			} else {
				em.SendText(in.text)
			}
		}
	}
}
