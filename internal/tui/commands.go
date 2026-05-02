package tui

import "strings"

// SlashAction identifies the slash command typed by the user.
type SlashAction string

const (
	SlashHelp    SlashAction = "help"
	SlashClear   SlashAction = "clear"
	SlashQuit    SlashAction = "quit"
	SlashUnknown SlashAction = "unknown"
)

// Slash command names accepted in V1 (Slice 2 minimal set). The full
// dispatcher with /model, /mcp, /skills, /memory, /stats lands in Slice 4.
var slashAliases = map[string]SlashAction{
	"help":  SlashHelp,
	"?":     SlashHelp,
	"clear": SlashClear,
	"quit":  SlashQuit,
	"exit":  SlashQuit,
	"q":     SlashQuit,
}

// ParseSlash inspects input. If it looks like a slash command (leading
// `/` after trimming whitespace), returns the recognized action, the
// raw command name (without leading `/`, as the user typed it for error
// messages), and isSlash=true. Otherwise returns ("", "", false).
//
// Unrecognized slash commands return SlashUnknown so callers can show a
// friendly "unknown command" message in the chat without leaking input
// to the model.
func ParseSlash(input string) (action SlashAction, command string, isSlash bool) {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "/") {
		return "", "", false
	}
	body := strings.TrimSpace(trimmed[1:])
	if body == "" {
		// Bare "/" — treat as unknown.
		return SlashUnknown, "", true
	}
	// First token only; any args (none in Slice 2) are ignored for now.
	cmd := strings.ToLower(strings.Fields(body)[0])
	if a, ok := slashAliases[cmd]; ok {
		return a, cmd, true
	}
	return SlashUnknown, cmd, true
}

// HelpText returns the multi-line help message printed by /help.
func HelpText() string {
	return strings.Join([]string{
		"Cogo — interactive mode",
		"",
		"Type a message and press Enter to send.",
		"Shift+Enter inserts a newline (multi-line input).",
		"",
		"Type / at the start of an empty prompt to open the slash-command palette.",
		"Type @ to open the file picker — selecting a file inserts @path/to/file,",
		"and Cogo inlines the file's contents when you send the message.",
		"",
		"Slash commands:",
		"  /help       show this help",
		"  /clear      clear chat history (asks for confirmation)",
		"  /quit       exit Cogo (alias: /exit)",
		"",
		"Keys:",
		"  PgUp/PgDn   scroll chat history",
		"  ↑/↓         recall previous prompts when input is empty",
		"              (cursor movement in the textarea otherwise)",
		"  Tab         in the slash/file palette: complete the highlighted",
		"              item without submitting (slash) or insert it (file)",
		"  Esc         dismiss the palette / permission prompt",
		"  Ctrl+C      cancel current turn (or exit when idle)",
		"  Ctrl+L      reset viewport scroll (history preserved)",
		"  Ctrl+U      clear the input box",
		"",
		"Mouse selection / copy / paste use your terminal's normal behavior.",
		"",
		"More commands (/model, /mcp, /skills, /memory, /stats) arrive in a later release.",
	}, "\n")
}
