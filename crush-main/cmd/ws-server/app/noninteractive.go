package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"charm.land/fantasy"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/rolling1314/rolling-crush/domain/message"
	"github.com/rolling1314/rolling-crush/internal/agent"
	"github.com/rolling1314/rolling-crush/internal/pkg/format"
	"github.com/rolling1314/rolling-crush/internal/pkg/term"
	"github.com/rolling1314/rolling-crush/internal/tui/components/anim"
	"github.com/rolling1314/rolling-crush/internal/tui/styles"
)

// RunNonInteractive runs the application in non-interactive mode with the
// given prompt, printing to stdout.
func (app *WSApp) RunNonInteractive(ctx context.Context, output io.Writer, prompt string, quiet bool) error {
	slog.Info("Running in non-interactive mode")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var spinner *format.Spinner
	if !quiet {
		t := styles.CurrentTheme()

		// Detect background color to set the appropriate color for the
		// spinner's 'Generating...' text. Without this, that text would be
		// unreadable in light terminals.
		hasDarkBG := true
		if f, ok := output.(*os.File); ok {
			hasDarkBG = lipgloss.HasDarkBackground(os.Stdin, f)
		}
		defaultFG := lipgloss.LightDark(hasDarkBG)(charmtone.Pepper, t.FgBase)

		spinner = format.NewSpinner(ctx, cancel, anim.Settings{
			Size:        10,
			Label:       "Generating",
			LabelColor:  defaultFG,
			GradColorA:  t.Primary,
			GradColorB:  t.Secondary,
			CycleColors: true,
		})
		spinner.Start()
	}

	// Helper function to stop spinner once.
	stopSpinner := func() {
		if !quiet && spinner != nil {
			spinner.Stop()
			spinner = nil
		}
	}
	defer stopSpinner()

	const maxPromptLengthForTitle = 100
	const titlePrefix = "Non-interactive: "
	var titleSuffix string

	if len(prompt) > maxPromptLengthForTitle {
		titleSuffix = prompt[:maxPromptLengthForTitle] + "..."
	} else {
		titleSuffix = prompt
	}
	title := titlePrefix + titleSuffix

	sess, err := app.Sessions.Create(ctx, "", title)
	if err != nil {
		return fmt.Errorf("failed to create session for non-interactive mode: %w", err)
	}
	slog.Info("Created session for non-interactive run", "session_id", sess.ID)

	// Automatically approve all permission requests for this non-interactive
	// session.
	app.Permissions.AutoApproveSession(sess.ID)

	type response struct {
		result *fantasy.AgentResult
		err    error
	}
	done := make(chan response, 1)

	go func(ctx context.Context, sessionID, prompt string) {
		result, err := app.AgentCoordinator.Run(ctx, sess.ID, prompt)
		if err != nil {
			done <- response{
				err: fmt.Errorf("failed to start agent processing stream: %w", err),
			}
		}
		done <- response{
			result: result,
		}
	}(ctx, sess.ID, prompt)

	messageEvents := app.Messages.Subscribe(ctx)
	messageReadBytes := make(map[string]int)
	supportsProgressBar := term.SupportsProgressBar()

	defer func() {
		if supportsProgressBar {
			_, _ = fmt.Fprintf(os.Stderr, ansi.ResetProgressBar)
		}

		// Always print a newline at the end. If output is a TTY this will
		// prevent the prompt from overwriting the last line of output.
		_, _ = fmt.Fprintln(output)
	}()

	for {
		if supportsProgressBar {
			// HACK: Reinitialize the terminal progress bar on every iteration so
			// it doesn't get hidden by the terminal due to inactivity.
			_, _ = fmt.Fprintf(os.Stderr, ansi.SetIndeterminateProgressBar)
		}

		select {
		case result := <-done:
			stopSpinner()
			if result.err != nil {
				if errors.Is(result.err, context.Canceled) || errors.Is(result.err, agent.ErrRequestCancelled) {
					slog.Info("Non-interactive: agent processing cancelled", "session_id", sess.ID)
					return nil
				}
				return fmt.Errorf("agent processing failed: %w", result.err)
			}
			return nil

		case event := <-messageEvents:
			msg := event.Payload
			if msg.SessionID == sess.ID && msg.Role == message.Assistant && len(msg.Parts) > 0 {
				stopSpinner()

				content := msg.Content().String()
				readBytes := messageReadBytes[msg.ID]

				if len(content) < readBytes {
					slog.Error("Non-interactive: message content is shorter than read bytes", "message_length", len(content), "read_bytes", readBytes)
					return fmt.Errorf("message content is shorter than read bytes: %d < %d", len(content), readBytes)
				}

				part := content[readBytes:]
				fmt.Fprint(output, part)
				messageReadBytes[msg.ID] = len(content)
			}

		case <-ctx.Done():
			stopSpinner()
			return ctx.Err()
		}
	}
}
