package status

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/lipgloss/v2/compat"
	"github.com/fsnotify/fsnotify"
	"github.com/sst/opencode/internal/app"
	"github.com/sst/opencode/internal/commands"
	"github.com/sst/opencode/internal/layout"
	"github.com/sst/opencode/internal/styles"
	"github.com/sst/opencode/internal/theme"
	"github.com/sst/opencode/internal/util"
)

type GitBranchUpdatedMsg struct {
	Branch string
}

type StatusComponent interface {
	tea.Model
	tea.ViewModel
	Cleanup()
}

type statusComponent struct {
	app        *app.App
	width      int
	cwd        string
	branch     string
	watcher    *fsnotify.Watcher
	done       chan struct{}
	lastUpdate time.Time
}

func (m *statusComponent) Init() tea.Cmd {
	return m.startGitWatcher()
}

func (m *statusComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case GitBranchUpdatedMsg:
		if m.branch != msg.Branch {
			m.branch = msg.Branch
		}
		// Continue watching for changes (persistent watcher)
		return m, m.watchForGitChanges()
	}
	return m, nil
}

func (m *statusComponent) logo() string {
	t := theme.CurrentTheme()
	base := styles.NewStyle().Foreground(t.TextMuted()).Background(t.BackgroundElement()).Render
	emphasis := styles.NewStyle().
		Foreground(t.Text()).
		Background(t.BackgroundElement()).
		Bold(true).
		Render

	open := base("open")
	code := emphasis("code")
	version := base(" " + m.app.Version)

	content := open + code
	if m.width > 40 {
		content += version
	}
	return styles.NewStyle().
		Background(t.BackgroundElement()).
		Padding(0, 1).
		Render(content)
}

func (m *statusComponent) collapsePath(path string, maxWidth int) string {
	if lipgloss.Width(path) <= maxWidth {
		return path
	}

	const ellipsis = ".."
	ellipsisLen := len(ellipsis)

	if maxWidth <= ellipsisLen {
		if maxWidth > 0 {
			return "..."[:maxWidth]
		}
		return ""
	}

	separator := string(filepath.Separator)
	parts := strings.Split(path, separator)

	if len(parts) == 1 {
		return path[:maxWidth-ellipsisLen] + ellipsis
	}

	truncatedPath := parts[len(parts)-1]
	for i := len(parts) - 2; i >= 0; i-- {
		part := parts[i]
		if len(truncatedPath)+len(separator)+len(part)+ellipsisLen > maxWidth {
			return ellipsis + separator + truncatedPath
		}
		truncatedPath = part + separator + truncatedPath
	}
	return truncatedPath
}

func (m *statusComponent) View() string {
	t := theme.CurrentTheme()
	logo := m.logo()
	logoWidth := lipgloss.Width(logo)

	var modeBackground compat.AdaptiveColor
	var modeForeground compat.AdaptiveColor

	agentColor := util.GetAgentColor(m.app.AgentIndex)

	if m.app.AgentIndex == 0 {
		modeBackground = t.BackgroundElement()
		modeForeground = agentColor
	} else {
		modeBackground = agentColor
		modeForeground = t.BackgroundPanel()
	}

	command := m.app.Commands[commands.SwitchAgentCommand]
	kb := command.Keybindings[0]
	key := kb.Key
	if kb.RequiresLeader {
		key = m.app.Config.Keybinds.Leader + " " + kb.Key
	}

	agentStyle := styles.NewStyle().Background(modeBackground).Foreground(modeForeground)
	agentNameStyle := agentStyle.Bold(true).Render
	agentDescStyle := agentStyle.Render
	agent := agentNameStyle(strings.ToUpper(m.app.Agent().Name)) + agentDescStyle(" AGENT")
	agent = agentStyle.
		Padding(0, 1).
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(modeBackground).
		BorderBackground(t.BackgroundPanel()).
		Render(agent)

	faintStyle := styles.NewStyle().
		Faint(true).
		Background(t.BackgroundPanel()).
		Foreground(t.TextMuted())
	agent = faintStyle.Render(key+" ") + agent
	modeWidth := lipgloss.Width(agent)

	availableWidth := m.width - logoWidth - modeWidth
	branchSuffix := ""
	if m.branch != "" {
		branchSuffix = ":" + m.branch
	}

	maxCwdWidth := availableWidth - lipgloss.Width(branchSuffix)
	cwdDisplay := m.collapsePath(m.cwd, maxCwdWidth)

	if m.branch != "" && availableWidth > lipgloss.Width(cwdDisplay)+lipgloss.Width(branchSuffix) {
		cwdDisplay += faintStyle.Render(branchSuffix)
	}

	cwd := styles.NewStyle().
		Foreground(t.TextMuted()).
		Background(t.BackgroundPanel()).
		Padding(0, 1).
		Render(cwdDisplay)

	background := t.BackgroundPanel()
	status := layout.Render(
		layout.FlexOptions{
			Background: &background,
			Direction:  layout.Row,
			Justify:    layout.JustifySpaceBetween,
			Align:      layout.AlignStretch,
			Width:      m.width,
		},
		layout.FlexItem{
			View: logo + cwd,
		},
		layout.FlexItem{
			View: agent,
		},
	)

	blank := styles.NewStyle().Background(t.Background()).Width(m.width).Render("")
	return blank + "\n" + status
}

func (m *statusComponent) startGitWatcher() tea.Cmd {
	cmd := util.CmdHandler(
		GitBranchUpdatedMsg{Branch: getCurrentGitBranch(m.app.Info.Path.Root)},
	)
	if err := m.initWatcher(); err != nil {
		return cmd
	}
	return tea.Batch(cmd, m.watchForGitChanges())
}

func (m *statusComponent) initWatcher() error {
	gitDir := filepath.Join(m.app.Info.Path.Root, ".git")
	headFile := filepath.Join(gitDir, "HEAD")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// Watch the entire .git directory instead of just HEAD so that atomic
	// rewrites of HEAD or refs are always detected across platforms.
	if err := watcher.Add(gitDir); err != nil {
		watcher.Close()
		return err
	}

	// Also watch the current ref file (in case of direct hash vs ref switches)
	refFile := getGitRefFile(m.app.Info.Path.Root)
	if refFile != headFile && refFile != "" {
		if _, err := os.Stat(refFile); err == nil {
			_ = watcher.Add(refFile) // ignore error: directory watch usually sufficient
		}
	}

	m.watcher = watcher
	m.done = make(chan struct{})
	return nil
}

func (m *statusComponent) watchForGitChanges() tea.Cmd {
	if m.watcher == nil {
		return nil
	}

	return tea.Cmd(func() tea.Msg {
		for {
			select {
			case event, ok := <-m.watcher.Events:
				if !ok {
					return GitBranchUpdatedMsg{Branch: getCurrentGitBranch(m.app.Info.Path.Root)}
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
					// Small delay to allow git to finish updating refs
					time.Sleep(60 * time.Millisecond)
					// Drain any burst of subsequent events without extra sleeps
				DrainLoop:
					for {
						select {
						case e := <-m.watcher.Events:
							_ = e // ignore, we just want to collapse bursts
							continue
						case <-time.After(5 * time.Millisecond):
							break DrainLoop
						}
					}
					if strings.HasSuffix(event.Name, "HEAD") {
						m.updateWatchedFiles()
					}
					return GitBranchUpdatedMsg{Branch: getCurrentGitBranch(m.app.Info.Path.Root)}
				}
			case <-m.watcher.Errors:
				// ignore errors, keep watching
			case <-m.done:
				return GitBranchUpdatedMsg{Branch: ""}
			}
		}
	})
}

func (m *statusComponent) updateWatchedFiles() {
	if m.watcher == nil {
		return
	}
	refFile := getGitRefFile(m.app.Info.Path.Root)
	headFile := filepath.Join(m.app.Info.Path.Root, ".git", "HEAD")
	if refFile != headFile && refFile != "" {
		if _, err := os.Stat(refFile); err == nil {
			// Try to add the new ref file (ignore error if already watching)
			m.watcher.Add(refFile)
		}
	}
}

func getCurrentGitBranch(cwd string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func getGitRefFile(cwd string) string {
	headFile := filepath.Join(cwd, ".git", "HEAD")
	content, err := os.ReadFile(headFile)
	if err != nil {
		return ""
	}

	headContent := strings.TrimSpace(string(content))
	if after, ok := strings.CutPrefix(headContent, "ref: "); ok {
		// HEAD points to a ref file
		refPath := after
		return filepath.Join(cwd, ".git", refPath)
	}

	// HEAD contains a direct commit hash
	return headFile
}

func (m *statusComponent) Cleanup() {
	if m.done != nil {
		close(m.done)
	}
	if m.watcher != nil {
		m.watcher.Close()
	}
}

func NewStatusCmp(app *app.App) StatusComponent {
	statusComponent := &statusComponent{
		app:        app,
		lastUpdate: time.Time{},
	}

	homePath, err := os.UserHomeDir()
	cwdPath := app.Info.Path.Cwd
	if err == nil && homePath != "" && strings.HasPrefix(cwdPath, homePath) {
		cwdPath = "~" + cwdPath[len(homePath):]
	}
	statusComponent.cwd = cwdPath

	return statusComponent
}
