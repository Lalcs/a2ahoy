package chat

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/presenter"
	"github.com/a2aproject/a2a-go/v2/a2a"
)

// chatPrompt is the input prompt rendered at the start of each REPL turn.
const chatPrompt = "> "

// chatMaxLineBytes caps the bufio.Scanner buffer to 1 MiB per line.
// The default 64 KiB limit is low enough that a large pasted prompt
// can trip bufio.ErrTooLong; 1 MiB is generous for any realistic
// interactive use while still bounding memory.
const chatMaxLineBytes = 1 << 20

// RunSimple runs the chat REPL in line-mode using bufio.Scanner.
//
// Simple mode is the IME-safe fallback: the terminal stays in cooked
// mode, so IME composition (CJK input methods) is delegated to the OS
// and cannot be broken by raw-mode key interception. It is also the
// mode selected automatically when --json is set, since JSON streaming
// output is incompatible with a full-screen TUI.
//
// ctx is the top-level context from the caller (usually a
// signal.NotifyContext in cmd.runChat). Each turn derives a fresh
// per-turn signal context so Ctrl+C during streaming cancels only the
// in-flight request and returns to the prompt, while Ctrl+C at the
// prompt exits the REPL via the default Go signal handler.
func RunSimple(ctx context.Context, c client.A2AClient, card *a2a.AgentCard, baseURL string, useJSON bool, initialParts []*a2a.Part, tenant string, cfg *a2a.SendMessageConfig) error {
	printWelcomeBanner(os.Stdout, card, baseURL)

	state := &State{}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), chatMaxLineBytes)

	for {
		_, _ = fmt.Fprint(os.Stdout, chatPrompt)
		if !scanner.Scan() {
			// EOF (Ctrl+D) or scanner error.
			if err := scanner.Err(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "input error: %v\n", err)
				return err
			}
			_, _ = fmt.Fprintln(os.Stdout) // newline after ^D for a clean shell prompt
			return nil
		}

		text, isSlash, sc := ParseInputLine(scanner.Text())
		if text == "" {
			continue // empty input → re-prompt
		}

		if isSlash {
			exit, err := handleSlashSimple(ctx, c, state, sc, os.Stdout, os.Stderr, useJSON)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			if exit {
				return nil
			}
			continue
		}

		// Regular message → streaming turn. Attach initial file parts
		// on the first turn only, then clear them.
		if err := runSimpleTurn(ctx, c, state, text, useJSON, initialParts, tenant, cfg); err != nil {
			// Errors print but do NOT exit the REPL unless the top-level
			// ctx was cancelled (user hit Ctrl+C at the outer level, not
			// via per-turn signal, or EOF upstream). A per-turn cancel
			// returns nil so we never land here for that case.
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				return nil
			}
			_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		initialParts = nil // consumed on first turn
	}
}

// runSimpleTurn performs one streaming turn: builds the request, sets
// up a per-turn signal-notify context, iterates the event stream, and
// updates state with the last observed TaskInfo on success.
func runSimpleTurn(ctx context.Context, c client.A2AClient, state *State, text string, useJSON bool, extraParts []*a2a.Part, tenant string, cfg *a2a.SendMessageConfig) error {
	req := BuildChatRequest(state, text, tenant, cfg, extraParts...)

	// Per-turn signal.NotifyContext: installs a SIGINT handler for the
	// duration of this turn only. turnCancel() below uninstalls it so
	// the next scanner.Scan() runs under Go's default SIGINT (which
	// terminates the process), matching the "Ctrl+C at prompt exits"
	// behaviour specified in the plan.
	turnCtx, turnCancel := signal.NotifyContext(ctx, os.Interrupt)
	defer turnCancel()

	// Accumulate IDs into a tmp State so cancellation can leave the
	// caller's state untouched. State.Update already implements the
	// merge-on-non-empty semantics we need.
	var tmp State
	for event, err := range c.SendStreamingMessage(turnCtx, req) {
		if err != nil {
			if turnCtx.Err() != nil {
				_, _ = fmt.Fprintln(os.Stderr, "\nInterrupted.")
				return nil // cancelled; state intentionally not updated
			}
			return fmt.Errorf("stream error: %w", err)
		}

		if useJSON {
			if perr := presenter.PrintJSON(os.Stdout, event); perr != nil {
				return perr
			}
		} else {
			if perr := presenter.PrintStreamEvent(os.Stdout, event); perr != nil {
				return perr
			}
		}

		tmp.Update(event.TaskInfo())
	}

	// Ensure spacing between turns even if the last event did not emit a
	// trailing newline (artifact-update events, for example).
	_, _ = fmt.Fprintln(os.Stdout)

	if !tmp.IsFresh() {
		state.Update(tmp.TaskInfo())
	}
	return nil
}

// handleSlashSimple dispatches a slash command in simple mode.
// Returns (shouldExit, err). Non-exit errors are printed by the caller.
func handleSlashSimple(ctx context.Context, c client.A2AClient, state *State, sc SlashCmd, stdout, stderr io.Writer, useJSON bool) (bool, error) {
	switch sc.Name {
	case "exit", "quit":
		return true, nil
	case "help":
		printChatHelp(stdout)
		return false, nil
	case "new":
		state.Reset()
		_, _ = fmt.Fprintln(stdout, "Started a new conversation.")
		return false, nil
	case "get":
		return false, runSimpleTaskCmd(ctx, state, sc.Arg, "get", stdout, useJSON,
			func(ctx context.Context, id a2a.TaskID) (*a2a.Task, error) {
				return c.GetTask(ctx, &a2a.GetTaskRequest{ID: id})
			})
	case "cancel":
		return false, runSimpleTaskCmd(ctx, state, sc.Arg, "cancel", stdout, useJSON,
			func(ctx context.Context, id a2a.TaskID) (*a2a.Task, error) {
				return c.CancelTask(ctx, &a2a.CancelTaskRequest{ID: id})
			})
	default:
		return false, fmt.Errorf("unknown command %q (try /help)", "/"+sc.Name)
	}
}

// runSimpleTaskCmd implements the /get and /cancel verbs in simple mode.
// The two slash commands differ only in which a2a-go RPC they call, so
// the dispatch lives in the caller and this helper handles ID
// resolution, error wrapping, and output formatting in one place.
func runSimpleTaskCmd(
	ctx context.Context,
	state *State,
	arg, verb string,
	stdout io.Writer,
	useJSON bool,
	fetch func(context.Context, a2a.TaskID) (*a2a.Task, error),
) error {
	targetID, err := state.ResolveTaskID(arg, verb)
	if err != nil {
		return err
	}
	task, err := fetch(ctx, targetID)
	if err != nil {
		return fmt.Errorf("tasks/%s failed: %w", verb, err)
	}
	if useJSON {
		return presenter.PrintJSON(stdout, task)
	}
	return presenter.PrintTask(stdout, task)
}

// printWelcomeBanner prints a short banner at REPL start. Kept minimal
// on purpose; the agent card can be fully inspected via `a2ahoy card`.
func printWelcomeBanner(w io.Writer, card *a2a.AgentCard, baseURL string) {
	name := "Agent"
	if card != nil && card.Name != "" {
		name = card.Name
	}
	_, _ = fmt.Fprintf(w, "Connected to %s (%s)\n", name, baseURL)
	_, _ = fmt.Fprintln(w, "Type a message and press Enter. Type /help for commands.")
	_, _ = fmt.Fprintln(w, "Press Ctrl+C at the prompt to exit.")
}

// printChatHelp prints the slash command reference. Shared between the
// simple-mode `/help` command and (when imported) the TUI help overlay
// so there is a single source of truth for command documentation.
func printChatHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Commands:")
	for _, s := range AllSuggestions {
		_, _ = fmt.Fprintf(w, "  %-10s %s\n", s.Name, s.Help)
	}
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "During streaming, Ctrl+C cancels the request and returns to the prompt.")
	_, _ = fmt.Fprintln(w, "At the prompt, Ctrl+C or Ctrl+D exits the chat.")
}
