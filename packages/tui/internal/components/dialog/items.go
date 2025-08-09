package dialog

import (
	"fmt"
	"github.com/sst/opencode/internal/app"
	"github.com/sst/opencode/internal/styles"
	"github.com/sst/opencode/internal/theme"
)

// ResourceItem is a unified type for both tools and agents in dialogs
type ResourceItem struct {
	Name           string
	DisplayName    string
	Type           string // "tool" or "agent"
	Source         string // "builtin"/"mcp" for tools, "subagent"/"primary"/"all" for agents
	Enabled        bool
	DefaultEnabled bool
	Overridden     bool // differs from default
	IsSelectable   bool
	Mode           string // for agents: "subagent", "primary", or "all"
	Description    string // optional description
	IsToggleMode   bool   // true for tool/agent toggles, false for agent selection
}

// NewToolResourceItem creates a ResourceItem for tools
func NewToolResourceItem(name, source string, enabled, defaultEnabled bool) ResourceItem {
	return ResourceItem{
		Name:           name,
		DisplayName:    name,
		Type:           "tool",
		Source:         source,
		Enabled:        enabled,
		DefaultEnabled: defaultEnabled,
		Overridden:     enabled != defaultEnabled,
		IsSelectable:   true,
		IsToggleMode:   true,
	}
}

// NewAgentResourceItem creates a ResourceItem for agents (for selection dialogs)
func NewAgentResourceItem(name, description, mode string, isToggleMode bool) ResourceItem {
	displayName := name
	if description == "" && mode != "" {
		description = fmt.Sprintf("(%s)", mode)
	}

	return ResourceItem{
		Name:         name,
		DisplayName:  displayName,
		Type:         "agent",
		Source:       mode,
		Mode:         mode,
		Description:  description,
		IsSelectable: true,
		IsToggleMode: isToggleMode,
	}
}

// NewAgentToggleResourceItem creates a ResourceItem for agent toggle functionality
func NewAgentToggleResourceItem(name, mode string, enabled, defaultEnabled bool) ResourceItem {
	return ResourceItem{
		Name:           name,
		DisplayName:    name,
		Type:           "agent",
		Source:         mode,
		Mode:           mode,
		Enabled:        enabled,
		DefaultEnabled: defaultEnabled,
		Overridden:     enabled != defaultEnabled,
		IsSelectable:   true,
		IsToggleMode:   true,
	}
}

func (r ResourceItem) Render(selected bool, width int, baseStyle styles.Style) string {
	theme := theme.CurrentTheme()

	itemStyle := baseStyle.
		Background(theme.BackgroundPanel()).
		Foreground(theme.Text())

	if selected {
		itemStyle = itemStyle.Foreground(theme.Primary())
	} else if r.Overridden && r.IsToggleMode {
		// non-selected overridden items get warning color in toggle mode
		itemStyle = itemStyle.Foreground(theme.Warning())
	}

	// For selection dialogs (agent selection), show description
	if r.Type == "agent" && !r.IsToggleMode {
		return r.renderSelection(itemStyle, baseStyle, width)
	}

	// For toggle dialogs (tools and agent toggles), show toggle state
	if r.IsToggleMode {
		toggleIndicator := "[ ]"
		if r.Enabled {
			toggleIndicator = "[✓]"
		}
		text := fmt.Sprintf("%s %s", toggleIndicator, r.DisplayName)
		return itemStyle.PaddingLeft(1).Render(text)
	}

	// Default rendering for other cases
	return itemStyle.PaddingLeft(1).Render(r.DisplayName)
}

func (r ResourceItem) renderSelection(itemStyle, baseStyle styles.Style, width int) string {
	descStyle := baseStyle.
		Foreground(theme.CurrentTheme().TextMuted()).
		Background(theme.CurrentTheme().BackgroundPanel())

	// Calculate available width (accounting for padding and margins)
	availableWidth := width - 2 // Account for left padding

	agentName := r.DisplayName
	description := r.Description
	separator := " - "

	// Calculate how much space we have for the description
	nameAndSeparatorLength := len(agentName) + len(separator)
	descriptionMaxLength := availableWidth - nameAndSeparatorLength

	// Truncate description if it's too long
	if len(description) > descriptionMaxLength && descriptionMaxLength > 3 {
		description = description[:descriptionMaxLength-3] + "..."
	}

	namePart := itemStyle.Render(agentName)
	descPart := descStyle.Render(separator + description)
	combinedText := namePart + descPart

	return baseStyle.PaddingLeft(1).Render(combinedText)
}

func (r ResourceItem) Selectable() bool {
	return r.IsSelectable
}

// IsResourceDefaultEnabled returns whether a resource is enabled by default
func IsResourceDefaultEnabled(resource app.ResourceInfo) bool {
	if resource.DefaultEnabled != nil {
		return *resource.DefaultEnabled
	}
	return true
}
