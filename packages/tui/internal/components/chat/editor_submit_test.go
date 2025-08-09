package chat

import (
	"testing"

	"github.com/sst/opencode/internal/app"
	"github.com/sst/opencode/internal/commands"
)

func TestSubmitCompactWithRemainder(t *testing.T) {
	ed := newTestEditor()
	fakeApp := &app.App{State: app.NewState()}
	fakeApp.Commands = commands.CommandRegistry{}
	fakeApp.Commands[commands.SessionCompactCommand] = commands.Command{
		Name:    commands.SessionCompactCommand,
		Trigger: []string{"compact", "summarize"},
	}
	ed.app = fakeApp
	ed.SetValue("/compact tell me how this relates to the color blue")
	_, cmd := ed.Submit()
	if cmd == nil {
		return
	}
	_ = cmd()
	if len(fakeApp.State.MessageHistory) != 0 {
		// Remainder should not be recorded as separate prompt
		if len(fakeApp.State.MessageHistory) == 1 {
			t.Fatalf("expected no prompt history entries, got 1: %v", fakeApp.State.MessageHistory[0])
		}
		t.Fatalf("expected no prompt history entries, got %d", len(fakeApp.State.MessageHistory))
	}
}
