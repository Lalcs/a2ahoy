package chat

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/a2aproject/a2a-go/v2/a2a"
)

// roleUser etc. name the semantic sender for each rendered message.
// Used as map-like keys for styling; kept as string constants so
// unknown roles can fall through to a default style.
const (
	roleUser   = "user"
	roleAgent  = "agent"
	roleSystem = "system"
	roleError  = "error"
)

// renderedMessage is a single user-visible chat entry shown in the
// viewport. One user turn plus its streamed agent response typically
// produce two renderedMessage entries: one "user" and one "agent".
//
// styled holds the lipgloss-formatted version of the entry. It is
// computed once per message (or, for streaming agent responses,
// recomputed only on the tail entry). Caching here avoids
// re-running lipgloss styling for every prior message on every
// streaming token.
type renderedMessage struct {
	role   string // roleUser | roleAgent | roleSystem | roleError
	text   string
	styled string
}

// Model is the Bubble Tea state for the chat TUI. It owns everything
// visible in the interface and all the bookkeeping needed to drive
// streaming turns and slash commands.
//
// Bubble Tea's Update loop returns a new Model value on every call,
// so all fields are either simple values, pointers (for shared state
// like streamCh), or reassignable.
type Model struct {
	// Program-level cancellation context. Used by slash-command RPC
	// closures so an in-flight /get or /cancel is cancelled when the
	// user quits the TUI, instead of running to its own timeout.
	ctx context.Context

	// Injected dependencies.
	client    client.A2AClient
	agentCard *a2a.AgentCard
	baseURL   string

	// Shared chat state: taskId / contextId used for continuation.
	state State

	// Rendered conversation history for the viewport.
	messages []renderedMessage

	// UI components.
	viewport  viewport.Model
	textInput textinput.Model
	// spinner animates while a streaming turn is in flight so the
	// user can distinguish "request stuck" from "request waiting".
	// Driven by spinner.TickMsg in Update; rendered in the status bar.
	spinner spinner.Model

	// Suggestion dropdown state.
	showSuggestions bool
	suggestions     []SuggestionItem
	selectedSugg    int

	// Streaming bookkeeping. While streaming == true:
	//   - streamCh receives events from the goroutine
	//   - streamCancel cancels the in-flight SendStreamingMessage
	//   - streamBuf accumulates agent response text for the rendered message
	//   - lastTurnInfo merges all TaskInfo values seen so far this turn
	streaming    bool
	streamCancel context.CancelFunc
	streamCh     chan streamEventInternal
	streamBuf    strings.Builder
	lastTurnInfo a2a.TaskInfo

	// Layout.
	width, height int
	ready         bool // true once we have received at least one WindowSizeMsg

	// Transient error line rendered below the input. Cleared on the
	// next successful action so one-off errors don't linger forever.
	errMsg string
}

// newModel constructs a Model wired to the given client and card.
// The textinput is focused immediately so the user can start typing
// without any additional key press.
func newModel(ctx context.Context, c client.A2AClient, card *a2a.AgentCard, baseURL string) Model {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.Placeholder = "Type a message or / for commands"
	ti.CharLimit = 0
	ti.Focus()

	// Viewport dimensions are set on the first WindowSizeMsg; zero
	// is fine until then because the View function checks m.ready.
	vp := viewport.New()
	vp.SoftWrap = true
	vp.MouseWheelEnabled = true

	// MiniDot is the smallest preset (single braille cell) so it sits
	// flush in the status bar without shifting the surrounding text.
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	sp.Style = spinnerStyle

	return Model{
		ctx:       ctx,
		client:    c,
		agentCard: card,
		baseURL:   baseURL,
		viewport:  vp,
		textInput: ti,
		spinner:   sp,
	}
}

// Init satisfies tea.Model. Returns a nil Cmd because the textinput is
// already focused in newModel and there is no initial async work.
func (m Model) Init() tea.Cmd {
	return nil
}

// appendMessage pushes a new rendered entry and re-renders the viewport
// content. Call this whenever the conversation transcript grows.
func (m *Model) appendMessage(role, text string) {
	m.messages = append(m.messages, renderedMessage{
		role:   role,
		text:   text,
		styled: styleMessage(role, text),
	})
	m.updateViewportContent()
}

// setLastAgentText overwrites the text of the current streaming agent
// message (the last entry in m.messages). If no agent message exists
// yet for this turn, a new one is appended. Used by the stream
// handlers to incrementally update the on-screen response without
// appending a new row per token.
func (m *Model) setLastAgentText(text string) {
	if n := len(m.messages); n > 0 && m.messages[n-1].role == roleAgent {
		m.messages[n-1].text = text
		m.messages[n-1].styled = styleMessage(roleAgent, text)
	} else {
		m.messages = append(m.messages, renderedMessage{
			role:   roleAgent,
			text:   text,
			styled: styleMessage(roleAgent, text),
		})
	}
	m.updateViewportContent()
}

// styleMessage returns the lipgloss-styled rendering for a single
// message. Splitting it out from updateViewportContent lets the model
// cache the result on each entry so prior messages do not get
// re-styled on every streaming token.
func styleMessage(role, text string) string {
	switch role {
	case roleUser:
		return userPrefixStyle.Render("you") + "  " + text
	case roleAgent:
		return agentPrefixStyle.Render("agent") + "  " + text
	case roleSystem:
		return systemMsgStyle.Render("· " + text)
	case roleError:
		return errLineStyle.Render("⚠ " + text)
	default:
		return text
	}
}

// updateViewportContent joins the cached styled strings of every
// message and pushes the result into the viewport, scrolling to the
// bottom. Each entry is already pre-styled by appendMessage or
// setLastAgentText, so this loop only does string concatenation —
// no lipgloss work is repeated for unchanged entries.
func (m *Model) updateViewportContent() {
	var sb strings.Builder
	for i, msg := range m.messages {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(msg.styled)
	}
	m.viewport.SetContent(sb.String())
	m.viewport.GotoBottom()
}

// updateSuggestions re-computes the dropdown contents based on the
// current textinput value. Called after every key press that may
// have mutated the input. Triggers a layout recalc so the viewport
// makes room for (or reclaims space from) the dropdown.
func (m *Model) updateSuggestions() {
	val := m.textInput.Value()
	// Show suggestions only while typing the command name itself;
	// once a space is entered we assume the user is typing an argument
	// and hide the dropdown.
	if strings.HasPrefix(val, "/") && !strings.Contains(val, " ") {
		m.suggestions = FilterSuggestions(val)
		m.showSuggestions = len(m.suggestions) > 0
		if m.selectedSugg >= len(m.suggestions) {
			m.selectedSugg = 0
		}
	} else {
		m.showSuggestions = false
		m.suggestions = nil
		m.selectedSugg = 0
	}
	m.recalcLayout()
}

// acceptSuggestion replaces the textinput value with the currently
// highlighted suggestion and a trailing space, then hides the
// dropdown so subsequent characters are treated as argument text.
// No-op if the selection is out of range.
func (m *Model) acceptSuggestion() {
	if m.selectedSugg < 0 || m.selectedSugg >= len(m.suggestions) {
		return
	}
	m.textInput.SetValue(m.suggestions[m.selectedSugg].Name + " ")
	m.textInput.CursorEnd()
	m.showSuggestions = false
	m.recalcLayout()
}

// agentName returns the agent card name or a generic fallback, used
// in the header banner.
func (m Model) agentName() string {
	if m.agentCard != nil && m.agentCard.Name != "" {
		return m.agentCard.Name
	}
	return "Agent"
}
