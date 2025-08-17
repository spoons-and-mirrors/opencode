package spinner

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/sst/opencode/internal/theme"
)

// TickMsg is sent when the spinner should update to the next frame
type TickMsg struct {
	Time time.Time
}

// OpenCodeSpinner renders frame-based animation
type OpenCodeSpinner struct {
	currentFrame int
	frames       []string
	interval     time.Duration
	isAnimating  bool
}

// New creates a new frame-based spinner
func New() *OpenCodeSpinner {
	// 5 frames of "O" animation moving up and down
	frames := []string{
		// Frame 1 - O in middle
		`........
..xxxx..
..x..x..
..x..x..
..xxxx..
........`,
		// Frame 2 - O moves down
		`........
........
..xxxx..
..x..x..
..x..x..
..xxxx..`,
		// Frame 3 - O at bottom with expansion
		`........
........
........
..xxxx..
..x..x..
.xxxxxx.`,
		// Frame 4 - O moves back up
		`........
..xxxx..
..x..x..
..x..x..
..xxxx..
........`,
		// Frame 5 - O at top
`..xxxx..
..x..x..
..x..x..
..xxxx..
........
........`,
`.xxxxxx.
..x..x..
..x..x..
...xx...
........
........`,
`..xxxx..
..x..x..
..xxxx..
........
........
........`,
`..xxxx..
..x..x..
..xxxx..
........
........
........`,
`........
..xxxx..
..x..x..
..xxxx..
........
........`,
	}

	return &OpenCodeSpinner{
		currentFrame: 0,
		frames:       frames,
		interval:     100 * time.Millisecond, // 10fps
		isAnimating:  false,
	}
}

// Init starts the spinner animation
func (s *OpenCodeSpinner) Init() tea.Cmd {
	s.isAnimating = true
	return s.tick()
}

// tick returns a command that will send a TickMsg after the interval
func (s *OpenCodeSpinner) tick() tea.Cmd {
	return tea.Tick(s.interval, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}

// Update handles messages for the spinner
func (s *OpenCodeSpinner) Update(msg tea.Msg) (*OpenCodeSpinner, tea.Cmd) {
	switch msg.(type) {
	case TickMsg:
		if !s.isAnimating {
			return s, nil
		}
		// Advance to next frame
		s.currentFrame = (s.currentFrame + 1) % len(s.frames)
		return s, s.tick()
	}
	return s, nil
}

// Start begins the animation
func (s *OpenCodeSpinner) Start() tea.Cmd {
	s.isAnimating = true
	return s.tick()
}

// Stop ends the animation
func (s *OpenCodeSpinner) Stop() {
	s.isAnimating = false
}

// renderFrame converts a frame string to styled output
func (s *OpenCodeSpinner) renderFrame(frame string) string {
	t := theme.CurrentTheme()

	var builder strings.Builder
	lines := strings.Split(frame, "\n")

	for i, line := range lines {
		for _, char := range line {
			switch char {
			case 'x':
				// Render ▀ character with primary color
				style := lipgloss.NewStyle().Foreground(t.Primary())
				builder.WriteString(style.Render("▀"))
			case '.':
				// Empty space
				builder.WriteString(" ")
			default:
				// Any other character renders as-is
				builder.WriteString(string(char))
			}
		}
		if i < len(lines)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// View renders the current frame
func (s *OpenCodeSpinner) View() string {
	if len(s.frames) == 0 {
		return ""
	}
	return s.renderFrame(s.frames[s.currentFrame])
}

// ViewO renders just the O (for compatibility)
func (s *OpenCodeSpinner) ViewO() string {
	return s.View()
}

// ViewC renders a simple "C" for compatibility
func (s *OpenCodeSpinner) ViewC() string {
	t := theme.CurrentTheme()
	style := lipgloss.NewStyle().Foreground(t.Primary())
	return style.Render(`▀▀
▀ 
▀ 
▀▀`)
}

// ViewOCOnly renders just the animation
func (s *OpenCodeSpinner) ViewOCOnly() string {
	return s.View()
}

// IsAnimating returns whether the spinner is currently animating
func (s *OpenCodeSpinner) IsAnimating() bool {
	return s.isAnimating
}

// SetInterval changes the animation speed
func (s *OpenCodeSpinner) SetInterval(interval time.Duration) {
	s.interval = interval
}

// LoadFrames allows loading custom frames
func (s *OpenCodeSpinner) LoadFrames(frames []string) {
	s.frames = frames
	s.currentFrame = 0
}
