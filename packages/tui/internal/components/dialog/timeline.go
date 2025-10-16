package dialog

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/muesli/reflow/truncate"
	"github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode/internal/app"
	"github.com/sst/opencode/internal/components/list"
	"github.com/sst/opencode/internal/components/modal"
	"github.com/sst/opencode/internal/layout"
	"github.com/sst/opencode/internal/styles"
	"github.com/sst/opencode/internal/theme"
	"github.com/sst/opencode/internal/util"
)

// TimelineDialog interface for the session timeline dialog
type TimelineDialog interface {
	layout.Modal
}

// ScrollToMessageMsg is sent when a message should be scrolled to
type ScrollToMessageMsg struct {
	MessageID string
}

// ScrollToPartMsg is sent when a part should be scrolled to
type ScrollToPartMsg struct {
	PartID string
}

// RestoreToMessageMsg is sent when conversation should be restored to a specific message
type RestoreToMessageMsg struct {
	MessageID string
	PartID    string // Optional: restore to specific part within message
	Index     int
}

// ForkFromMessageMsg is sent when a new session should be forked from a specific message
type ForkFromMessageMsg struct {
	MessageID string
	Index     int
}

// timelineItem represents a user message in the timeline list (normal mode)
type timelineItem struct {
	messageID string
	content   string
	timestamp time.Time
	index     int // Index in the full message list
	toolCount int // Number of tools used in this message
}

func (n timelineItem) Render(
	selected bool,
	width int,
	isFirstInViewport bool,
	baseStyle styles.Style,
	isCurrent bool,
) string {
	t := theme.CurrentTheme()
	infoStyle := baseStyle.Background(t.BackgroundPanel()).Foreground(t.Info()).Render
	textStyle := baseStyle.Background(t.BackgroundPanel()).Foreground(t.Text()).Render

	// Add dot after timestamp if this is the current message - only apply color when not selected
	var dot string
	var dotVisualLen int
	if isCurrent {
		if selected {
			dot = "● "
		} else {
			dot = lipgloss.NewStyle().Foreground(t.Success()).Render("● ")
		}
		dotVisualLen = 2 // "● " is 2 characters wide
	}

	// Format timestamp - only apply color when not selected
	var timeStr string
	var timeVisualLen int
	if selected {
		timeStr = n.timestamp.Format("15:04") + " " + dot
		timeVisualLen = lipgloss.Width(n.timestamp.Format("15:04")+" ") + dotVisualLen
	} else {
		timeStr = infoStyle(n.timestamp.Format("15:04")+" ") + dot
		timeVisualLen = lipgloss.Width(n.timestamp.Format("15:04")+" ") + dotVisualLen
	}

	// Tool count display (fixed width for alignment) - only apply color when not selected
	toolInfo := ""
	toolInfoVisualLen := 0
	if n.toolCount > 0 {
		toolInfoText := fmt.Sprintf("(%d tools)", n.toolCount)
		if selected {
			toolInfo = toolInfoText
		} else {
			toolInfo = infoStyle(toolInfoText)
		}
		toolInfoVisualLen = lipgloss.Width(toolInfo)
	}

	// Calculate available space for content
	// Reserve space for: timestamp + dot + space + toolInfo + padding + some buffer
	reservedSpace := timeVisualLen + 1 + toolInfoVisualLen + 4
	contentWidth := max(width-reservedSpace, 8)

	truncatedContent := truncate.StringWithTail(
		strings.Split(n.content, "\n")[0],
		uint(contentWidth),
		"...",
	)

	// Apply normal text color to content for non-selected items
	var styledContent string
	if selected {
		styledContent = truncatedContent
	} else {
		styledContent = textStyle(truncatedContent)
	}

	// Create the line with proper spacing - content left-aligned, tools right-aligned
	var text string
	text = timeStr + styledContent
	if toolInfo != "" {
		bgColor := t.BackgroundPanel()
		if selected {
			bgColor = t.Primary()
		}
		text = layout.Render(
			layout.FlexOptions{
				Background: &bgColor,
				Direction:  layout.Row,
				Justify:    layout.JustifySpaceBetween,
				Align:      layout.AlignStretch,
				Width:      width - 2,
			},
			layout.FlexItem{
				View: text,
			},
			layout.FlexItem{
				View: toolInfo,
			},
		)
	}

	var itemStyle styles.Style
	if selected {
		itemStyle = baseStyle.
			Background(t.Primary()).
			Foreground(t.BackgroundElement()).
			Width(width).
			PaddingLeft(1)
	} else {
		itemStyle = baseStyle.PaddingLeft(1)
	}

	return itemStyle.Render(text)
}

func (n timelineItem) Selectable() bool {
	return true
}

// atomicItem represents a single part in the timeline (atomic mode)
type atomicItem struct {
	partID       string
	messageID    string
	content      string
	timestamp    *time.Time // Only for user text parts
	partType     string     // "user_text", "assistant_text", "reasoning", "tool"
	messageIndex int
	partIndex    int
	isUserText   bool
	toolCount    int // Number of tools in parent user message (for display)
}

func (n atomicItem) Render(
	selected bool,
	width int,
	isFirstInViewport bool,
	baseStyle styles.Style,
	isCurrent bool,
) string {
	t := theme.CurrentTheme()
	infoStyle := baseStyle.Background(t.BackgroundPanel()).Foreground(t.Info()).Render
	textStyle := baseStyle.Background(t.BackgroundPanel()).Foreground(t.Text()).Render

	var timeStr string
	var timeVisualLen int

	if n.isUserText && n.timestamp != nil {
		// User text: show timestamp
		if selected {
			timeStr = n.timestamp.Format("15:04") + " "
		} else {
			timeStr = infoStyle(n.timestamp.Format("15:04") + " ")
		}
		timeVisualLen = lipgloss.Width(n.timestamp.Format("15:04") + " ")
	} else {
		// All other parts: show dot marker with same spacing as timestamp
		if selected {
			timeStr = "    • "
		} else {
			timeStr = "    " + infoStyle("• ")
		}
		timeVisualLen = 6
	}

	// Tool count display (fixed width for alignment)
	toolInfo := ""
	toolInfoVisualLen := 0
	if n.toolCount > 0 && n.isUserText {
		toolInfoText := fmt.Sprintf("(%d tools)", n.toolCount)
		if selected {
			toolInfo = toolInfoText
		} else {
			toolInfo = infoStyle(toolInfoText)
		}
		toolInfoVisualLen = lipgloss.Width(toolInfo)
	}

	// Calculate available space for content
	reservedSpace := timeVisualLen + 1 + toolInfoVisualLen + 4
	contentWidth := max(width-reservedSpace, 8)

	truncatedContent := truncate.StringWithTail(
		strings.Split(n.content, "\n")[0],
		uint(contentWidth),
		"...",
	)

	// Apply normal text color to content for non-selected items
	var styledContent string
	if selected {
		styledContent = truncatedContent
	} else {
		styledContent = textStyle(truncatedContent)
	}

	// Create the line with proper spacing - content left-aligned, tools right-aligned
	var text string
	text = timeStr + styledContent
	if toolInfo != "" {
		bgColor := t.BackgroundPanel()
		if selected {
			bgColor = t.Primary()
		}
		text = layout.Render(
			layout.FlexOptions{
				Background: &bgColor,
				Direction:  layout.Row,
				Justify:    layout.JustifySpaceBetween,
				Align:      layout.AlignStretch,
				Width:      width - 2,
			},
			layout.FlexItem{
				View: text,
			},
			layout.FlexItem{
				View: toolInfo,
			},
		)
	}

	var itemStyle styles.Style
	if selected {
		itemStyle = baseStyle.
			Background(t.Primary()).
			Foreground(t.BackgroundElement()).
			Width(width).
			PaddingLeft(1)
	} else {
		itemStyle = baseStyle.PaddingLeft(1)
	}

	return itemStyle.Render(text)
}

func (n atomicItem) Selectable() bool {
	return true
}

type timelineDialog struct {
	width      int
	height     int
	modal      *modal.Modal
	normalList list.List[timelineItem]
	atomicList list.List[atomicItem]
	app        *app.App
	viewMode   string // "normal" or "atomic"
}

func (n *timelineDialog) Init() tea.Cmd {
	return nil
}

func (n *timelineDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		n.width = msg.Width
		n.height = msg.Height
		n.normalList.SetMaxWidth(layout.Current.Container.Width - 12)
		n.atomicList.SetMaxWidth(layout.Current.Container.Width - 12)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "a":
			// Toggle view mode
			if n.viewMode == "normal" {
				n.viewMode = "atomic"
				n.modal.SetTitle("Session Timeline (Atomic)")
			} else {
				n.viewMode = "normal"
				n.modal.SetTitle("Session Timeline (Normal)")
			}
			return n, nil
		case "up", "down":
			// Handle navigation based on current mode
			if n.viewMode == "atomic" {
				listModel, cmd := n.atomicList.Update(msg)
				n.atomicList = listModel.(list.List[atomicItem])

				// Get the newly selected item and scroll to the specific part
				if item, idx := n.atomicList.GetSelectedItem(); idx >= 0 {
					return n, tea.Sequence(
						cmd,
						util.CmdHandler(ScrollToPartMsg{PartID: item.partID}),
					)
				}
				return n, cmd
			} else {
				listModel, cmd := n.normalList.Update(msg)
				n.normalList = listModel.(list.List[timelineItem])

				// Get the newly selected item and scroll to it immediately
				if item, idx := n.normalList.GetSelectedItem(); idx >= 0 {
					return n, tea.Sequence(
						cmd,
						util.CmdHandler(ScrollToMessageMsg{MessageID: item.messageID}),
					)
				}
				return n, cmd
			}
		case "r":
			// Restore conversation to selected message
			if n.viewMode == "atomic" {
				if item, idx := n.atomicList.GetSelectedItem(); idx >= 0 {
					// In atomic mode, restore to the specific part
					// Backend's splitWhen excludes the matched part, so we need to pass the NEXT part
					nextPartID := n.findNextPartID(item.messageIndex, item.partIndex)
					if nextPartID == "" {
						// This is the last part, restore to end of session (no messageID)
						return n, tea.Sequence(
							util.CmdHandler(RestoreToMessageMsg{MessageID: "", Index: item.messageIndex}),
							util.CmdHandler(modal.CloseModalMsg{}),
						)
					} else {
						return n, tea.Sequence(
							util.CmdHandler(RestoreToMessageMsg{MessageID: item.messageID, PartID: nextPartID, Index: item.messageIndex}),
							util.CmdHandler(modal.CloseModalMsg{}),
						)
					}
				}
			} else {
				if item, idx := n.normalList.GetSelectedItem(); idx >= 0 {
					// In normal mode, restore to the entire user message
					// Backend's splitWhen excludes the matched message, so we need to pass the NEXT message
					nextMessageID := n.findNextMessageID(item.index)
					if nextMessageID == "" {
						// This is the last message, restore to end of session (no messageID)
						return n, tea.Sequence(
							util.CmdHandler(RestoreToMessageMsg{MessageID: "", Index: item.index}),
							util.CmdHandler(modal.CloseModalMsg{}),
						)
					} else {
						return n, tea.Sequence(
							util.CmdHandler(RestoreToMessageMsg{MessageID: nextMessageID, Index: item.index}),
							util.CmdHandler(modal.CloseModalMsg{}),
						)
					}
				}
			}
		case "f":
			// Fork session from selected message/part
			if n.viewMode == "atomic" {
				if item, idx := n.atomicList.GetSelectedItem(); idx >= 0 {
					// In atomic mode, fork from the exact message containing this part
					// Backend forks up to (but not including) the messageID
					// So to include the selected part's message, we need to find the NEXT message
					nextMessageID := n.findNextMessageID(item.messageIndex)
					return n, tea.Sequence(
						util.CmdHandler(ForkFromMessageMsg{MessageID: nextMessageID, Index: item.messageIndex}),
						util.CmdHandler(modal.CloseModalMsg{}),
					)
				}
			} else {
				if item, idx := n.normalList.GetSelectedItem(); idx >= 0 {
					// In normal mode, fork including the full assistant response
					endIndex := n.findResponseEndIndex(item.index)
					return n, tea.Sequence(
						util.CmdHandler(ForkFromMessageMsg{MessageID: item.messageID, Index: endIndex}),
						util.CmdHandler(modal.CloseModalMsg{}),
					)
				}
			}
		case "enter":
			// Keep Enter functionality for closing the modal
			return n, util.CmdHandler(modal.CloseModalMsg{})
		}
	}

	// Update the appropriate list based on view mode
	var cmd tea.Cmd
	if n.viewMode == "atomic" {
		listModel, listCmd := n.atomicList.Update(msg)
		n.atomicList = listModel.(list.List[atomicItem])
		cmd = listCmd
	} else {
		listModel, listCmd := n.normalList.Update(msg)
		n.normalList = listModel.(list.List[timelineItem])
		cmd = listCmd
	}
	return n, cmd
}

func (n *timelineDialog) Render(background string) string {
	var listView string
	if n.viewMode == "atomic" {
		listView = n.atomicList.View()
	} else {
		listView = n.normalList.View()
	}

	t := theme.CurrentTheme()
	keyStyle := styles.NewStyle().
		Foreground(t.Text()).
		Background(t.BackgroundPanel()).
		Bold(true).
		Render
	mutedStyle := styles.NewStyle().Foreground(t.TextMuted()).Background(t.BackgroundPanel()).Render

	helpText := keyStyle(
		"↑/↓",
	) + mutedStyle(
		" jump   ",
	) + keyStyle(
		"r",
	) + mutedStyle(
		" restore   ",
	) + keyStyle(
		"f",
	) + mutedStyle(
		" fork   ",
	) + keyStyle(
		"a",
	) + mutedStyle(
		" atomic",
	)

	bgColor := t.BackgroundPanel()
	helpView := styles.NewStyle().
		Background(bgColor).
		Width(layout.Current.Container.Width - 14).
		PaddingLeft(1).
		PaddingTop(1).
		Render(helpText)

	content := strings.Join([]string{listView, helpView}, "\n")

	return n.modal.Render(content, background)
}

func (n *timelineDialog) Close() tea.Cmd {
	return nil
}

// extractMessagePreview extracts a preview from message parts
func extractMessagePreview(parts []opencode.PartUnion) string {
	for _, part := range parts {
		switch casted := part.(type) {
		case opencode.TextPart:
			text := strings.TrimSpace(casted.Text)
			if text != "" {
				return text
			}
		}
	}
	return "No text content"
}

// extractPartPreview extracts a preview from a single part
func extractPartPreview(part opencode.PartUnion) string {
	switch casted := part.(type) {
	case opencode.TextPart:
		text := strings.TrimSpace(casted.Text)
		if text != "" {
			return text
		}
		return "Empty text"
	case opencode.ReasoningPart:
		text := strings.TrimSpace(casted.Text)
		if text != "" {
			return "Thinking: " + text
		}
		return "Thinking..."
	case opencode.ToolPart:
		if casted.Tool != "" {
			return fmt.Sprintf("Tool: %s", casted.Tool)
		}
		return "Tool call"
	default:
		return "Unknown part"
	}
}

// countToolsInResponse counts tools in the assistant's response to a user message
func countToolsInResponse(messages []app.Message, userMessageIndex int) int {
	count := 0
	// Look at subsequent messages to find the assistant's response
	for i := userMessageIndex + 1; i < len(messages); i++ {
		message := messages[i]
		// If we hit another user message, stop looking
		if _, isUser := message.Info.(opencode.UserMessage); isUser {
			break
		}
		// Count tools in this assistant message
		for _, part := range message.Parts {
			switch part.(type) {
			case opencode.ToolPart:
				count++
			}
		}
	}
	return count
}

// findResponseEndIndex finds the last message index of the assistant's response to a user message
func (n *timelineDialog) findResponseEndIndex(userMessageIndex int) int {
	messages := n.app.Messages
	endIndex := userMessageIndex // Default to the user message index

	// Look ahead to find the end of the assistant's response
	for i := userMessageIndex + 1; i < len(messages); i++ {
		message := messages[i]
		// If we hit another user message, stop (don't include it)
		if _, isUser := message.Info.(opencode.UserMessage); isUser {
			break
		}
		// This is part of the assistant's response, include it
		endIndex = i
	}

	return endIndex
}

// findNextMessageID finds the ID of the next message after the given index
// Returns empty string if this is the last message
func (n *timelineDialog) findNextMessageID(messageIndex int) string {
	messages := n.app.Messages
	if messageIndex+1 >= len(messages) {
		// This is the last message, return empty to fork entire session
		return ""
	}

	nextMessage := messages[messageIndex+1]
	switch casted := nextMessage.Info.(type) {
	case opencode.UserMessage:
		return casted.ID
	case opencode.AssistantMessage:
		return casted.ID
	}
	return ""
}

// findNextPartID finds the ID of the next valid part after the given message/part index
// Returns empty string if this is the last part in the session
func (n *timelineDialog) findNextPartID(messageIndex, partIndex int) string {
	messages := n.app.Messages
	message := messages[messageIndex]

	// First, look for next part in current message
	for i := partIndex + 1; i < len(message.Parts); i++ {
		part := message.Parts[i]
		if !isPartValid(part) {
			continue
		}
		switch casted := part.(type) {
		case opencode.TextPart:
			return casted.ID
		case opencode.ReasoningPart:
			return casted.ID
		case opencode.ToolPart:
			return casted.ID
		}
	}

	// If no part found in current message, look in subsequent messages
	for i := messageIndex + 1; i < len(messages); i++ {
		nextMessage := messages[i]
		for _, part := range nextMessage.Parts {
			if !isPartValid(part) {
				continue
			}
			switch casted := part.(type) {
			case opencode.TextPart:
				return casted.ID
			case opencode.ReasoningPart:
				return casted.ID
			case opencode.ToolPart:
				return casted.ID
			}
		}
	}

	// This is the last part, return empty
	return ""
}

// isPartValid checks if a part should be included in the atomic view
func isPartValid(part opencode.PartUnion) bool {
	switch casted := part.(type) {
	case opencode.TextPart:
		// Exclude synthetic and empty text parts
		return !casted.Synthetic && strings.TrimSpace(casted.Text) != ""
	case opencode.ReasoningPart:
		// Include reasoning parts with content
		return strings.TrimSpace(casted.Text) != ""
	case opencode.ToolPart:
		// Include all tool parts
		return true
	default:
		// Exclude other part types (file, patch, etc.)
		return false
	}
}

// buildAtomicItems creates a flat list of all parts across all messages
func buildAtomicItems(messages []app.Message) []atomicItem {
	var items []atomicItem

	// First pass: calculate tool counts for each user message
	toolCounts := make(map[int]int) // map user message index to tool count
	for i, message := range messages {
		if _, isUser := message.Info.(opencode.UserMessage); isUser {
			toolCounts[i] = countToolsInResponse(messages, i)
		}
	}

	// Track the last user message index for tool count lookup
	lastUserMsgIndex := -1

	for msgIndex, message := range messages {
		var messageID string
		var isUserMessage bool
		var userTimestamp *time.Time

		switch casted := message.Info.(type) {
		case opencode.UserMessage:
			messageID = casted.ID
			isUserMessage = true
			ts := time.UnixMilli(int64(casted.Time.Created))
			userTimestamp = &ts
			lastUserMsgIndex = msgIndex
		case opencode.AssistantMessage:
			messageID = casted.ID
			isUserMessage = false
		default:
			continue
		}

		// Get tool count from parent user message
		toolCount := 0
		if lastUserMsgIndex >= 0 {
			toolCount = toolCounts[lastUserMsgIndex]
		}

		for partIndex, part := range message.Parts {
			if !isPartValid(part) {
				continue
			}

			var partID, partType string
			var isUserText bool

			switch casted := part.(type) {
			case opencode.TextPart:
				partID = casted.ID
				if isUserMessage {
					partType = "user_text"
					isUserText = true
				} else {
					partType = "assistant_text"
				}
			case opencode.ReasoningPart:
				partID = casted.ID
				partType = "reasoning"
			case opencode.ToolPart:
				partID = casted.ID
				partType = "tool"
			}

			content := extractPartPreview(part)

			items = append(items, atomicItem{
				partID:    partID,
				messageID: messageID,
				content:   content,
				timestamp: func() *time.Time {
					if isUserText {
						return userTimestamp
					}
					return nil
				}(),
				partType:     partType,
				messageIndex: msgIndex,
				partIndex:    partIndex,
				isUserText:   isUserText,
				toolCount:    toolCount,
			})
		}
	}

	return items
}

// NewTimelineDialog creates a new session timeline dialog
func NewTimelineDialog(app *app.App) TimelineDialog {
	var normalItems []timelineItem

	// Build normal mode items (only user messages)
	for i, message := range app.Messages {
		if userMsg, ok := message.Info.(opencode.UserMessage); ok {
			preview := extractMessagePreview(message.Parts)
			toolCount := countToolsInResponse(app.Messages, i)

			normalItems = append(normalItems, timelineItem{
				messageID: userMsg.ID,
				content:   preview,
				timestamp: time.UnixMilli(int64(userMsg.Time.Created)),
				index:     i,
				toolCount: toolCount,
			})
		}
	}

	// Build atomic mode items (all parts)
	atomicItems := buildAtomicItems(app.Messages)

	normalListComponent := list.NewListComponent(
		list.WithItems(normalItems),
		list.WithMaxVisibleHeight[timelineItem](12),
		list.WithFallbackMessage[timelineItem]("No user messages in this session"),
		list.WithAlphaNumericKeys[timelineItem](true),
		list.WithRenderFunc(
			func(item timelineItem, selected bool, width int, baseStyle styles.Style) string {
				// Determine if this item is the current message for the session
				isCurrent := false
				if app.Session.Revert.MessageID != "" {
					// When reverted, Session.Revert.MessageID contains the NEXT user message ID
					// So we need to find the previous user message to highlight the correct one
					for i, navItem := range normalItems {
						if navItem.messageID == app.Session.Revert.MessageID && i > 0 {
							// Found the next message, so the previous one is current
							isCurrent = item.messageID == normalItems[i-1].messageID
							break
						}
					}
				} else if len(app.Messages) > 0 {
					// If not reverted, highlight the last user message
					lastUserMsgID := ""
					for i := len(app.Messages) - 1; i >= 0; i-- {
						if userMsg, ok := app.Messages[i].Info.(opencode.UserMessage); ok {
							lastUserMsgID = userMsg.ID
							break
						}
					}
					isCurrent = item.messageID == lastUserMsgID
				}
				// Only show the dot if undo/redo/restore is available
				showDot := app.Session.Revert.MessageID != ""
				return item.Render(selected, width, false, baseStyle, isCurrent && showDot)
			},
		),
		list.WithSelectableFunc(func(item timelineItem) bool {
			return true
		}),
	)
	normalListComponent.SetMaxWidth(layout.Current.Container.Width - 12)

	atomicListComponent := list.NewListComponent(
		list.WithItems(atomicItems),
		list.WithMaxVisibleHeight[atomicItem](12),
		list.WithFallbackMessage[atomicItem]("No parts in this session"),
		list.WithAlphaNumericKeys[atomicItem](true),
		list.WithRenderFunc(
			func(item atomicItem, selected bool, width int, baseStyle styles.Style) string {
				return item.Render(selected, width, false, baseStyle, false)
			},
		),
		list.WithSelectableFunc(func(item atomicItem) bool {
			return true
		}),
	)
	atomicListComponent.SetMaxWidth(layout.Current.Container.Width - 12)

	return &timelineDialog{
		normalList: normalListComponent,
		atomicList: atomicListComponent,
		app:        app,
		viewMode:   "normal",
		modal: modal.New(
			modal.WithTitle("Session Timeline (Normal)"),
			modal.WithMaxWidth(layout.Current.Container.Width-8),
		),
	}
}
