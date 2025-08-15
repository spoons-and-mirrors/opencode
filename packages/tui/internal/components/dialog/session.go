package dialog

import (
	"context"
	"strings"

	"slices"

	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/muesli/reflow/truncate"
	"github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode/internal/app"
	"github.com/sst/opencode/internal/components/list"
	"github.com/sst/opencode/internal/components/modal"
	"github.com/sst/opencode/internal/components/toast"
	"github.com/sst/opencode/internal/layout"
	"github.com/sst/opencode/internal/styles"
	"github.com/sst/opencode/internal/theme"
	"github.com/sst/opencode/internal/util"
)

// SessionDialog interface for the session switching dialog
type SessionDialog interface {
	layout.Modal
}

// sessionItem is a custom list item for sessions that can show delete confirmation
type sessionItem struct {
	id                 string
	title              string
	isDeleteConfirming bool
	isCurrentSession   bool
	isPinned           bool
}

func (s sessionItem) Render(
	selected bool,
	width int,
	isFirstInViewport bool,
	baseStyle styles.Style,
) string {
	t := theme.CurrentTheme()

	var text string
	if s.isDeleteConfirming {
		text = "Press again to confirm delete"
	} else {
		prefix := ""
		if s.isPinned {
			if selected {
				// When selected, use plain text to avoid color conflicts
				prefix = "● "
			} else {
				// When not selected, use Success color for the pin dot
				prefix = styles.NewStyle().Foreground(t.Success()).Render("● ")
			}
		}

		// Apply normal text color to title for non-selected items
		var styledTitle string
		if selected {
			styledTitle = s.title
		} else {
			textStyle := styles.NewStyle().Foreground(t.Text()).Render
			styledTitle = textStyle(s.title)
		}

		if s.isCurrentSession {
			// For current session, only show the current session dot, hide pin dot
			text = "● " + styledTitle
		} else {
			text = prefix + styledTitle
		}
	}

	truncatedStr := truncate.StringWithTail(text, uint(width-1), "...")

	var itemStyle styles.Style
	if selected {
		if s.isDeleteConfirming {
			// Red background for delete confirmation
			itemStyle = baseStyle.
				Background(t.Error()).
				Foreground(t.BackgroundElement()).
				Width(width).
				PaddingLeft(1)
		} else if s.isCurrentSession {
			// Different style for current session when selected
			itemStyle = baseStyle.
				Background(t.Primary()).
				Foreground(t.BackgroundElement()).
				Width(width).
				PaddingLeft(1).
				Bold(true)
		} else {
			// Normal selection
			itemStyle = baseStyle.
				Background(t.Primary()).
				Foreground(t.BackgroundElement()).
				Width(width).
				PaddingLeft(1)
		}
	} else {
		if s.isDeleteConfirming {
			// Red text for delete confirmation when not selected
			itemStyle = baseStyle.
				Foreground(t.Error()).
				PaddingLeft(1)
		} else if s.isCurrentSession {
			// Highlight current session when not selected
			itemStyle = baseStyle.
				Foreground(t.Primary()).
				PaddingLeft(1).
				Bold(true)
		} else {
			itemStyle = baseStyle.
				PaddingLeft(1)
		}
	}

	return itemStyle.Render(truncatedStr)
}

func (s sessionItem) Selectable() bool {
	return true
}

type sessionDialog struct {
	width      int
	height     int
	modal      *modal.Modal
	sessions   []opencode.Session
	list       list.List[sessionItem]
	app        *app.App
	confirmID  string // session ID for delete confirmation, empty means no confirmation
	renameMode bool
	input      textinput.Model
	index      int  // index of session being renamed
	pinnedOnly bool // true = show only pinned sessions, false = show all sessions
}

func (s *sessionDialog) Init() tea.Cmd {
	return nil
}

func (s *sessionDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.list.SetMaxWidth(layout.Current.Container.Width - 12)
	case tea.KeyPressMsg:
		if s.renameMode {
			switch msg.String() {
			case "enter":
				if _, idx := s.list.GetSelectedItem(); idx >= 0 && idx < len(s.sessions) && idx == s.index {
					title := s.input.Value()
					if strings.TrimSpace(title) != "" {
						session := s.sessions[idx]
						return s, tea.Sequence(
							func() tea.Msg {
								ctx := context.Background()
								err := s.app.UpdateSession(ctx, session.ID, title)
								if err != nil {
									return toast.NewErrorToast("Failed to rename session: " + err.Error())()
								}
								s.sessions[idx].Title = title
								s.renameMode = false
								s.modal.SetTitle("Switch Session")
								s.updateListItems()
								return toast.NewSuccessToast("Session renamed successfully")()
							},
						)
					}
				}
				s.renameMode = false
				s.modal.SetTitle("Switch Session")
				s.updateListItems()
				return s, nil
			default:
				var cmd tea.Cmd
				s.input, cmd = s.input.Update(msg)
				return s, cmd
			}
		} else {
			switch msg.String() {
			case "enter":
				if s.confirmID != "" {
					s.confirmID = ""
					s.updateListItems()
					return s, nil
				}
				if session := s.getSelectedSession(); session != nil {
					return s, tea.Sequence(
						util.CmdHandler(modal.CloseModalMsg{}),
						util.CmdHandler(app.SessionSelectedMsg(session)),
					)
				}
			case "n":
				return s, tea.Sequence(
					util.CmdHandler(modal.CloseModalMsg{}),
					util.CmdHandler(app.SessionClearedMsg{}),
				)
			case "r":
				if session := s.getSelectedSession(); session != nil {
					s.renameMode = true
					// Store the session ID instead of index for rename
					for i, sess := range s.sessions {
						if sess.ID == session.ID {
							s.index = i
							break
						}
					}
					s.setupRenameInput(session.Title)
					s.modal.SetTitle("Rename Session")
					s.updateListItems()
					return s, textinput.Blink
				}
			case "b":
				if session := s.getSelectedSession(); session != nil {
					newPinnedState := !session.Pinned
					return s, tea.Sequence(
						func() tea.Msg {
							ctx := context.Background()
							err := s.app.PinSession(ctx, session.ID, newPinnedState)
							if err != nil {
								return toast.NewErrorToast("Failed to pin/unpin session: " + err.Error())()
							}
							// Update the session in the main sessions list
							for i := range s.sessions {
								if s.sessions[i].ID == session.ID {
									s.sessions[i].Pinned = newPinnedState
									break
								}
							}
							s.updateListItems()
							pinText := "pinned"
							if !newPinnedState {
								pinText = "unpinned"
							}
							return toast.NewSuccessToast("Session " + pinText + " successfully")()
						},
					)
				}
			case "tab":
				s.pinnedOnly = !s.pinnedOnly
				if s.pinnedOnly {
					s.modal.SetTitle("Switch Session (bookmarks)")
				}
				if !s.pinnedOnly {
					s.modal.SetTitle("Switch Session")
				}
				s.updateListItems()
				return s, nil
			case "x", "delete", "backspace":
				if session := s.getSelectedSession(); session != nil {
					if s.confirmID == session.ID {
						// Second press - actually delete the session
						return s, tea.Sequence(
							func() tea.Msg {
								// Remove from sessions list
								for i, sess := range s.sessions {
									if sess.ID == session.ID {
										s.sessions = slices.Delete(s.sessions, i, i+1)
										break
									}
								}
								s.confirmID = ""
								s.updateListItems()
								return nil
							},
							s.deleteSession(session.ID),
						)
					} else {
						// First press - enter delete confirmation mode
						s.confirmID = session.ID
						s.updateListItems()
						return s, nil
					}
				}
			case "esc":
				if s.confirmID != "" {
					s.confirmID = ""
					s.updateListItems()
					return s, nil
				}
			}
		}
	}

	if !s.renameMode {
		var cmd tea.Cmd
		listModel, cmd := s.list.Update(msg)
		s.list = listModel.(list.List[sessionItem])
		return s, cmd
	}
	return s, nil
}

func (s *sessionDialog) Render(background string) string {
	if s.renameMode {
		// Show rename input instead of list
		t := theme.CurrentTheme()
		renameView := s.input.View()

		mutedStyle := styles.NewStyle().Foreground(t.TextMuted()).Background(t.BackgroundPanel()).Render
		helpText := mutedStyle("Enter to confirm, Esc to cancel")
		helpText = styles.NewStyle().PaddingLeft(1).PaddingTop(1).Render(helpText)

		content := strings.Join([]string{renameView, helpText}, "\n")
		return s.modal.Render(content, background)
	}

	listView := s.list.View()

	t := theme.CurrentTheme()
	// Use warning color for shortcut keys to make them stand out
	keyStyle := styles.NewStyle().Foreground(t.Warning()).Background(t.BackgroundPanel()).Render
	mutedStyle := styles.NewStyle().Foreground(t.TextMuted()).Background(t.BackgroundPanel()).Render

	leftHelp := keyStyle("n") + mutedStyle(" new") + "   " + keyStyle("r") + mutedStyle(" rename") + "   " + keyStyle("b") + mutedStyle(" bookmark") + "   " + keyStyle("tab") + mutedStyle(" filter")
	rightHelp := keyStyle("x/del") + mutedStyle(" delete")

	bgColor := t.BackgroundPanel()
	helpText := layout.Render(layout.FlexOptions{
		Direction:  layout.Row,
		Justify:    layout.JustifySpaceBetween,
		Width:      layout.Current.Container.Width - 14,
		Background: &bgColor,
	}, layout.FlexItem{View: leftHelp}, layout.FlexItem{View: rightHelp})

	helpText = styles.NewStyle().PaddingLeft(1).PaddingTop(1).Render(helpText)

	content := strings.Join([]string{listView, helpText}, "\n")

	return s.modal.Render(content, background)
}

func (s *sessionDialog) setupRenameInput(currentTitle string) {
	t := theme.CurrentTheme()
	bgColor := t.BackgroundPanel()
	textColor := t.Text()
	textMutedColor := t.TextMuted()

	s.input = textinput.New()
	s.input.SetValue(currentTitle)
	s.input.Focus()
	s.input.CharLimit = 100
	s.input.SetWidth(layout.Current.Container.Width - 20)

	s.input.Styles.Blurred.Placeholder = styles.NewStyle().
		Foreground(textMutedColor).
		Background(bgColor).
		Lipgloss()
	s.input.Styles.Blurred.Text = styles.NewStyle().
		Foreground(textColor).
		Background(bgColor).
		Lipgloss()
	s.input.Styles.Focused.Placeholder = styles.NewStyle().
		Foreground(textMutedColor).
		Background(bgColor).
		Lipgloss()
	s.input.Styles.Focused.Text = styles.NewStyle().
		Foreground(textColor).
		Background(bgColor).
		Lipgloss()
	s.input.Styles.Focused.Prompt = styles.NewStyle().
		Background(bgColor).
		Lipgloss()
}

func (s *sessionDialog) getSelectedSession() *opencode.Session {
	item, idx := s.list.GetSelectedItem()
	if idx < 0 {
		return nil
	}

	// Find the session by ID
	for i := range s.sessions {
		if s.sessions[i].ID == item.id {
			return &s.sessions[i]
		}
	}
	return nil
}

func (s *sessionDialog) updateListItems() {
	_, idx := s.list.GetSelectedItem()

	// Filter sessions based on pinned view mode
	var filtered []opencode.Session
	if s.pinnedOnly {
		for _, sess := range s.sessions {
			if sess.Pinned {
				filtered = append(filtered, sess)
			}
		}
		// Only sort in pinned-only view - sort by title
		slices.SortFunc(filtered, func(a, b opencode.Session) int {
			return strings.Compare(a.Title, b.Title)
		})
	} else {
		// In regular view, maintain original order (no sorting)
		filtered = s.sessions
	}

	var items []sessionItem
	for _, sess := range filtered {
		item := sessionItem{
			id:                 sess.ID,
			title:              sess.Title,
			isDeleteConfirming: s.confirmID == sess.ID,
			isCurrentSession:   s.app.Session != nil && s.app.Session.ID == sess.ID,
			isPinned:           sess.Pinned,
		}
		items = append(items, item)
	}
	s.list.SetItems(items)

	// Adjust selected index if necessary
	if idx >= len(items) && len(items) > 0 {
		s.list.SetSelectedIndex(len(items) - 1)
	} else {
		s.list.SetSelectedIndex(idx)
	}
}

func (s *sessionDialog) deleteSession(sessionID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := s.app.DeleteSession(ctx, sessionID); err != nil {
			return toast.NewErrorToast("Failed to delete session: " + err.Error())()
		}
		return nil
	}
}

// ReopenSessionModalMsg is emitted when the session modal should be reopened
type ReopenSessionModalMsg struct{}

func (s *sessionDialog) Close() tea.Cmd {
	if s.renameMode {
		// If in rename mode, exit rename mode and return a command to reopen the modal
		s.renameMode = false
		s.modal.SetTitle("Switch Session")
		s.updateListItems()

		// Return a command that will reopen the session modal
		return func() tea.Msg {
			return ReopenSessionModalMsg{}
		}
	}
	// Normal close behavior
	return nil
}

// NewSessionDialog creates a new session switching dialog
func NewSessionDialog(app *app.App) SessionDialog {
	sessions, _ := app.ListSessions(context.Background())

	var filtered []opencode.Session
	var items []sessionItem
	for _, sess := range sessions {
		if sess.ParentID != "" {
			continue
		}
		filtered = append(filtered, sess)
		items = append(items, sessionItem{
			id:                 sess.ID,
			title:              sess.Title,
			isDeleteConfirming: false,
			isCurrentSession:   app.Session != nil && app.Session.ID == sess.ID,
			isPinned:           sess.Pinned,
		})
	}

	listComponent := list.NewListComponent(
		list.WithItems(items),
		list.WithMaxVisibleHeight[sessionItem](10),
		list.WithFallbackMessage[sessionItem]("No sessions available"),
		list.WithAlphaNumericKeys[sessionItem](true),
		list.WithRenderFunc(
			func(item sessionItem, selected bool, width int, baseStyle styles.Style) string {
				return item.Render(selected, width, false, baseStyle)
			},
		),
		list.WithSelectableFunc(func(item sessionItem) bool {
			return true
		}),
	)
	listComponent.SetMaxWidth(layout.Current.Container.Width - 12)

	return &sessionDialog{
		sessions:   filtered,
		list:       listComponent,
		app:        app,
		confirmID:  "",
		renameMode: false,
		index:      -1,
		pinnedOnly: false,
		modal: modal.New(
			modal.WithTitle("Switch Session"),
			modal.WithMaxWidth(layout.Current.Container.Width-8),
		),
	}
}
