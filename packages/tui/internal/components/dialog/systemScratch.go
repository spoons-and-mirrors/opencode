package dialog

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/sst/opencode/internal/app"
	"github.com/sst/opencode/internal/components/modal"
	"github.com/sst/opencode/internal/components/textarea"
	"github.com/sst/opencode/internal/layout"
	"github.com/sst/opencode/internal/styles"
	"github.com/sst/opencode/internal/theme"
	"github.com/sst/opencode/internal/util"
)

// SystemScratchUpdatedMsg is sent when system scratch content is updated
type SystemScratchUpdatedMsg struct {
	Content string
}

// SystemScratchDialog interface for the system scratch modal
type SystemScratchDialog interface {
	layout.Modal
	GetContent() string
	SetContent(content string)
}

type systemScratchDialog struct {
	width    int
	height   int
	modal    *modal.Modal
	textarea textarea.Model
	app      *app.App
}

func (n *systemScratchDialog) Init() tea.Cmd {
	return n.textarea.Focus()
}

func (n *systemScratchDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		n.width = msg.Width
		n.height = msg.Height
		// Update textarea width to fit modal
		n.textarea.SetWidth(layout.Current.Container.Width - 20)
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Save content before closing
			content := strings.TrimSpace(n.textarea.Value())
			return n, tea.Sequence(
				util.CmdHandler(modal.CloseModalMsg{}),
				util.CmdHandler(SystemScratchUpdatedMsg{Content: content}),
			)
		}
	}

	var cmd tea.Cmd
	n.textarea, cmd = n.textarea.Update(msg)
	return n, cmd
}

func (n *systemScratchDialog) Render(background string) string {
	view := n.textarea.View()
	helpText := styles.NewStyle().
		Foreground(theme.CurrentTheme().TextMuted()).
		Render("Press Esc to close and save")

	content := strings.Join([]string{view, "", helpText}, "\n")
	return n.modal.Render(content, background)
}

func (n *systemScratchDialog) Close() tea.Cmd {
	// Save content when closing
	content := strings.TrimSpace(n.textarea.Value())
	return util.CmdHandler(SystemScratchUpdatedMsg{Content: content})
}

func (n *systemScratchDialog) GetContent() string {
	return n.textarea.Value()
}

func (n *systemScratchDialog) SetContent(content string) {
	n.textarea.SetValue(content)
}

// NewSystemScratchDialog creates a new system scratch modal dialog
func NewSystemScratchDialog(app *app.App) SystemScratchDialog {
	t := theme.CurrentTheme()
	bgColor := t.BackgroundPanel()
	textColor := t.Text()
	textMutedColor := t.TextMuted()

	ta := textarea.New()
	ta.SetWidth(layout.Current.Container.Width - 20)
	ta.SetHeight(12)
	ta.Focus()
	ta.CharLimit = 5000
	ta.Placeholder = "Your session scratchpad...\n\nWrite anything here: todos, notes, ideas, system prompt extension etc. This scratchpad is saved with the session and is shared with the agent."

	// Style the textarea
	ta.Styles.Focused.CursorLine = styles.NewStyle().Background(bgColor).Lipgloss()
	ta.Styles.Blurred.CursorLine = styles.NewStyle().Background(bgColor).Lipgloss()
	ta.Styles.Focused.Base = styles.NewStyle().
		Foreground(textColor).
		Background(bgColor).
		Lipgloss()
	ta.Styles.Blurred.Base = styles.NewStyle().
		Foreground(textMutedColor).
		Background(bgColor).
		Lipgloss()

	return &systemScratchDialog{
		textarea: ta,
		modal:    modal.New(modal.WithTitle("Scratchpad"), modal.WithMaxWidth(90)),
		app:      app,
	}
}
