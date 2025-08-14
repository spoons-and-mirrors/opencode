package dialog

import (
	"context"
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode/internal/app"
	"github.com/sst/opencode/internal/components/list"
	"github.com/sst/opencode/internal/components/modal"
	"github.com/sst/opencode/internal/layout"
	"github.com/sst/opencode/internal/util"
)

const (
	numVisibleResources    = 20
	minResourceDialogWidth = 40
)

// ResourceDialog interface for unified resource management
type ResourceDialog interface{ layout.Modal }

type resourceDialog struct {
	app          *app.App
	resourceType string // "tool" or "agent"
	allResources []ResourceItem
	modal        *modal.Modal
	searchDialog *SearchDialog
	dialogWidth  int
	width        int
	height       int
}

func (d *resourceDialog) Init() tea.Cmd {
	if d.searchDialog != nil {
		return d.searchDialog.Init()
	}
	return nil
}

func (d *resourceDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SearchSelectionMsg:
		if item, ok := msg.Item.(ResourceItem); ok {
			if item.Type == "agent" && !item.IsToggleMode {
				// Agent selection mode
				agents := d.app.Agents
				for _, agent := range agents {
					if agent.Name == item.Name {
						return d, tea.Sequence(
							util.CmdHandler(modal.CloseModalMsg{}),
							util.CmdHandler(app.AgentSelectedMsg{AgentName: agent.Name}),
						)
					}
				}
			} else {
				// Toggle mode for tools or agent overrides
				d.toggleResource(item)
				return d, d.emitUpdateMessage(item)
			}
		}
		return d, nil
	case SearchCancelledMsg:
		return d, util.CmdHandler(modal.CloseModalMsg{})
	case SearchQueryChangedMsg:
		items := d.buildItems(msg.Query)
		if d.searchDialog != nil {
			d.searchDialog.SetItems(items)
		}
		return d, nil
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
		if d.searchDialog != nil {
			d.searchDialog.SetWidth(d.dialogWidth)
			d.searchDialog.SetHeight(msg.Height)
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return d, util.CmdHandler(modal.CloseModalMsg{})
		case "tab":
			if d.resourceType == "tool" {
				// Remember current selection before switching agents
				var selectedName string
				if d.searchDialog != nil {
					if selectedItem, _ := d.searchDialog.GetSelectedItem(); selectedItem != nil {
						if res, ok := selectedItem.(ResourceItem); ok {
							selectedName = res.Name
						}
					}
				}

				// Cycle to next agent (forward)
				updated, _ := d.app.SwitchAgent()
				d.app = updated
				d.setupAllResources()

				// Try to restore a reasonable selection after rebuild
				if selectedName != "" && d.searchDialog != nil {
					curQuery := d.searchDialog.GetQuery()
					items := d.buildItems(curQuery)
					d.searchDialog.SetItems(items)
					d.restoreSelectionByName(selectedName, items)
				}
			}
			return d, nil
		}
	}

	if d.searchDialog != nil {
		updatedDialog, cmd := d.searchDialog.Update(msg)
		d.searchDialog = updatedDialog.(*SearchDialog)
		return d, cmd
	}
	return d, nil
}

func (d *resourceDialog) View() string {
	if d.searchDialog == nil {
		return "Loading..."
	}
	return d.searchDialog.View()
}

func (d *resourceDialog) Render(background string) string {
	return d.modal.Render(d.View(), background)
}

func (d *resourceDialog) Close() tea.Cmd {
	return nil
}

func (d *resourceDialog) toggleResource(item ResourceItem) {
	// Remember the name of the currently selected item for restoration
	selectedName := item.Name

	for i := range d.allResources {
		if d.allResources[i].Name == item.Name && d.allResources[i].Type == item.Type {
			d.allResources[i].Enabled = !d.allResources[i].Enabled
			d.allResources[i].Overridden = d.allResources[i].Enabled != d.allResources[i].DefaultEnabled
			break
		}
	}

	// Rebuild visual list and restore selection
	curQuery := d.searchDialog.GetQuery()
	newItems := d.buildItems(curQuery)
	d.searchDialog.SetItems(newItems)

	// Restore selection by finding the item with the same name
	d.restoreSelectionByName(selectedName, newItems)
}

// restoreSelectionByName attempts to restore selection to an item with the given name
func (d *resourceDialog) restoreSelectionByName(name string, items []list.Item) {
	if name == "" {
		return
	}

	for i, item := range items {
		switch v := item.(type) {
		case ResourceItem:
			if v.Name == name {
				d.searchDialog.SetSelectedIndex(i)
				return
			}
		}
	}
}

func (d *resourceDialog) emitUpdateMessage(item ResourceItem) tea.Cmd {
	agent := d.app.Agent()

	if item.Type == "tool" {
		overrides := make(map[string]bool)
		for _, res := range d.allResources {
			if res.Type == "tool" && res.Overridden {
				overrides[res.Name] = res.Enabled
			}
		}
		return util.CmdHandler(app.ResourceUpdatedMsg{Agent: agent.Name, ResourceType: "tools", Overrides: overrides})
	} else if item.Type == "utils" {
		overrides := make(map[string]bool)
		for _, res := range d.allResources {
			if res.Type == "utils" && res.Overridden {
				overrides[res.Name] = res.Enabled
			}
		}
		return util.CmdHandler(app.ResourceUpdatedMsg{Agent: agent.Name, ResourceType: "utils", Overrides: overrides})
	} else {
		overrides := make(map[string]bool)
		for _, res := range d.allResources {
			if res.Type == "agent" && res.IsToggleMode {
				overrides[res.Name] = res.Enabled
			}
		}
		// For subagents, use current agent name (not the resource agent name)
		return util.CmdHandler(app.ResourceUpdatedMsg{Agent: agent.Name, ResourceType: "agents", Overrides: overrides})
	}
}

func (d *resourceDialog) setupAllResources() {

	ctx := context.Background()
	agent := d.app.Agent()

	// Always initialize allResources to prevent nil issues
	d.allResources = make([]ResourceItem, 0)

	if d.resourceType == "tool" {
		d.setupToolResources(ctx, agent)
	} else {
		d.setupAgentResources(ctx, agent)
	}

	// Calculate optimal width
	d.dialogWidth = minResourceDialogWidth
	maxWidth := 0
	for _, res := range d.allResources {
		itemWidth := len(res.DisplayName) + 10 // padding + toggle indicator
		if itemWidth > maxWidth {
			maxWidth = itemWidth
		}
	}
	if maxWidth > d.dialogWidth {
		d.dialogWidth = maxWidth
	}

	// Always initialize search dialog, even if resources failed to load
	if d.searchDialog == nil {
		title := fmt.Sprintf("Search %ss...", d.resourceType)
		d.searchDialog = NewSearchDialog(title, numVisibleResources)
		if d.searchDialog == nil {
			return
		}
	} else {
	}

	d.searchDialog.SetWidth(d.dialogWidth)

	// Build initial display list
	items := d.buildItems("")
	d.searchDialog.SetItems(items)
}

func (d *resourceDialog) setupToolResources(ctx context.Context, agent *opencode.Agent) {

	// Initialize allResources if not already done
	if d.allResources == nil {
		d.allResources = make([]ResourceItem, 0)
	}

	// Get tools
	availableTools, err := d.app.ListTools(ctx)
	if err != nil {
		// Add some dummy tools to test the dialog
		d.allResources = append(d.allResources, []ResourceItem{
			NewToolResourceItem("bash", "builtin", true, true),
			NewToolResourceItem("edit", "builtin", true, true),
			NewToolResourceItem("read", "builtin", true, true),
			NewToolResourceItem("write", "builtin", false, true),
		}...)
	} else {

		toolOverrides := d.app.GetSessionOverrides(agent.Name, "tool")

		// Build tool items with current state
		toolKeys := make([]string, 0, len(availableTools))
		for k, toolInfo := range availableTools {
			if k != "invalid" && (toolInfo.DefaultEnabled == nil || *toolInfo.DefaultEnabled) {
				toolKeys = append(toolKeys, k)
			}
		}
		sort.Strings(toolKeys)

		for _, toolName := range toolKeys {
			toolInfo := availableTools[toolName]
			defaultEnabled := IsToolDefaultEnabled(toolInfo)
			if agentSetting, exists := agent.Tools[toolName]; exists {
				defaultEnabled = agentSetting
			}
			enabled := defaultEnabled
			if override, exists := toolOverrides[toolName]; exists {
				enabled = override
			}

			d.allResources = append(d.allResources, NewToolResourceItem(
				toolName, toolInfo.Source, enabled, defaultEnabled,
			))
		}
	}

	// Add subagents as toggles (this was missing!)
	availableAgents, err := d.app.ListAgents(ctx)
	if err != nil {
		// Add some dummy agents for testing
		d.allResources = append(d.allResources, []ResourceItem{
			NewAgentToggleResourceItem("general", "subagent", true, true),
			NewAgentToggleResourceItem("docs", "subagent", true, true),
		}...)
	} else {

		agentOverrides := d.app.GetSessionOverrides(agent.Name, "agent")

		// Add subagents as toggles
		for _, agentInfo := range availableAgents {
			if agentInfo.Mode == "primary" {
				continue // Skip primary agents
			}

			defaultEnabled := true // subagents are enabled by default
			enabled := defaultEnabled
			if override, exists := agentOverrides[agentInfo.Name]; exists {
				enabled = override
			}

			d.allResources = append(d.allResources, NewAgentToggleResourceItem(
				agentInfo.Name, agentInfo.Mode, enabled, defaultEnabled,
			))
		}
	}

	// Add utils section (for auto-compact toggle, etc.)
	utilsOverrides := d.app.GetSessionOverrides(agent.Name, "utils")

	// Auto-compact setting
	autoCompactDefault := true // auto-compact is enabled by default
	autoCompactEnabled := autoCompactDefault
	if override, exists := utilsOverrides["auto compact"]; exists {
		autoCompactEnabled = override
	}

	d.allResources = append(d.allResources, NewUtilsResourceItem(
		"auto compact", autoCompactEnabled, autoCompactDefault,
	))

}

func (d *resourceDialog) setupAgentResources(ctx context.Context, agent *opencode.Agent) {

	// Initialize allResources if not already done
	if d.allResources == nil {
		d.allResources = make([]ResourceItem, 0)
	}

	// For now, add some dummy agents to test the dialog
	d.allResources = append(d.allResources, []ResourceItem{
		NewAgentResourceItem("general", "General-purpose agent", "subagent", false),
		NewAgentResourceItem("docs", "Documentation agent", "subagent", false),
	}...)

	// Try to get real agents but don't fail if it doesn't work
	availableAgents, err := d.app.ListAgents(ctx)
	if err != nil {
		return
	}

	// Clear dummy data and use real data
	d.allResources = make([]ResourceItem, 0)

	// For agent dialog, show selection mode (not toggle mode)
	for _, agentInfo := range availableAgents {
		if agentInfo.Mode == "primary" {
			continue // Skip primary agents in selection
		}

		d.allResources = append(d.allResources, NewAgentResourceItem(
			agentInfo.Name, agentInfo.Description, agentInfo.Mode, false, // false = selection mode
		))
	}
}

func (d *resourceDialog) buildItems(query string) []list.Item {
	if query == "" {
		return d.buildGroupedItems()
	}
	return d.buildSearchItems(query)
}

func (d *resourceDialog) buildGroupedItems() []list.Item {
	var items []list.Item

	if d.resourceType == "tool" {
		// Group by source/type with proper ordering: builtin tools, mcp tools, subagents, then utils

		// 1. Built-in Tools
		builtinTools := make([]ResourceItem, 0)
		mcpTools := make([]ResourceItem, 0)
		agents := make([]ResourceItem, 0)
		utils := make([]ResourceItem, 0)

		for _, res := range d.allResources {
			switch {
			case res.Type == "tool" && res.Source == "builtin":
				builtinTools = append(builtinTools, res)
			case res.Type == "tool" && res.Source == "mcp":
				mcpTools = append(mcpTools, res)
			case res.Type == "agent":
				agents = append(agents, res)
			case res.Type == "utils":
				utils = append(utils, res)
			}
		}

		// Sort each category alphabetically within itself
		sort.Slice(builtinTools, func(i, j int) bool {
			return builtinTools[i].Name < builtinTools[j].Name
		})
		sort.Slice(mcpTools, func(i, j int) bool {
			return mcpTools[i].Name < mcpTools[j].Name
		})
		sort.Slice(agents, func(i, j int) bool {
			return agents[i].Name < agents[j].Name
		})
		sort.Slice(utils, func(i, j int) bool {
			return utils[i].Name < utils[j].Name
		})

		// Add groups in order with headers
		if len(builtinTools) > 0 {
			items = append(items, list.HeaderItem("Built-in Tools"))
			for _, tool := range builtinTools {
				items = append(items, tool)
			}
		}

		if len(mcpTools) > 0 {
			items = append(items, list.HeaderItem("MCP Tools"))
			for _, tool := range mcpTools {
				items = append(items, tool)
			}
		}

		if len(agents) > 0 {
			items = append(items, list.HeaderItem("Subagents"))
			for _, agent := range agents {
				items = append(items, agent)
			}
		}

		if len(utils) > 0 {
			items = append(items, list.HeaderItem("Utilities"))
			for _, util := range utils {
				items = append(items, util)
			}
		}
	} else {
		// For agent selection dialog (not tools dialog), just list agents
		for _, res := range d.allResources {
			items = append(items, res)
		}
	}

	return items
}

func (d *resourceDialog) buildSearchItems(query string) []list.Item {
	var matches []ResourceItem

	for _, res := range d.allResources {
		// Check fuzzy match against multiple fields
		matched := false

		// Match against name and display name
		if fuzzy.MatchFold(query, res.Name) || fuzzy.MatchFold(query, res.DisplayName) {
			matched = true
		}

		// Match against category names
		if !matched {
			categoryName := ""
			switch {
			case res.Type == "tool" && res.Source == "builtin":
				categoryName = "builtin tools"
			case res.Type == "tool" && res.Source == "mcp":
				categoryName = "mcp tools"
			case res.Type == "agent":
				categoryName = "subagents agents"
			case res.Type == "utils":
				categoryName = "utils utilities"
			}

			if fuzzy.MatchFold(query, categoryName) || fuzzy.MatchFold(query, res.Source) {
				matched = true
			}
		}

		// Match against description if available
		if !matched && res.Description != "" {
			if fuzzy.MatchFold(query, res.Description) {
				matched = true
			}
		}

		if matched {
			matches = append(matches, res)
		}
	}

	// Sort matches by category first, then by name within category
	sort.Slice(matches, func(i, j int) bool {
		resI, resJ := matches[i], matches[j]

		// First, sort by type and source priority
		orderI := getResourceOrder(resI)
		orderJ := getResourceOrder(resJ)

		if orderI != orderJ {
			return orderI < orderJ
		}

		// Within same category, sort by name
		return resI.Name < resJ.Name
	})

	// Convert to list items
	items := make([]list.Item, len(matches))
	for i, match := range matches {
		items[i] = match
	}

	return items
}

// Helper function to determine sort order for resources
func getResourceOrder(res ResourceItem) int {
	switch {
	case res.Type == "tool" && res.Source == "builtin":
		return 1
	case res.Type == "tool" && res.Source == "mcp":
		return 2
	case res.Type == "agent":
		return 3
	case res.Type == "utils":
		return 4
	default:
		return 5
	}
}

// Factory functions
func NewToolsDialog(app *app.App) ResourceDialog {
	dialog := &resourceDialog{
		app:          app,
		resourceType: "tool",
	}

	// Setup resources immediately in constructor, like other dialogs
	dialog.setupAllResources()

	dialog.modal = modal.New(modal.WithTitle("Toolbox"))
	return dialog
}

func NewAgentsDialog(app *app.App) ResourceDialog {
	dialog := &resourceDialog{
		app:          app,
		resourceType: "agent",
	}

	// Setup resources immediately in constructor, like other dialogs
	dialog.setupAllResources()

	dialog.modal = modal.New(modal.WithTitle("Select Agent"))
	return dialog
}

// Legacy types for compatibility - these will be removed when agents.go is updated
type ToolsDialog interface{ layout.Modal }
