package spinner

import (
	"math"
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

// OpenCodeSpinner is a morphing O↔C spinner with trailing and easing
type OpenCodeSpinner struct {
	progress      float64       // 0.0–2.0, cycles O→C→O
	interval      time.Duration // ms per frame
	cycleDuration float64       // seconds for a full O→C→O cycle
	trailLength   int           // how many trailing steps
	isAnimating   bool
}

// New creates a new morphing O↔C spinner
func New() *OpenCodeSpinner {
	return &OpenCodeSpinner{
		progress:      0,
		interval:      60 * time.Millisecond,
		cycleDuration: 2.2, // seconds for full O→C→O
		trailLength:   5,
		isAnimating:   false,
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
		// Advance progress
		step := s.interval.Seconds() / s.cycleDuration
		s.progress += step
		if s.progress >= 2.0 {
			s.progress -= 2.0
		}
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

// View renders the current frame of the spinner (both O and C)
func (s *OpenCodeSpinner) View() string {
	return lipgloss.JoinHorizontal(lipgloss.Top, s.ViewO(), "  ", s.ViewC())
}

// ViewOCOnly renders just the O and C letters together without any other text
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

// --- Animation logic ---

// O and C share a 2x4 grid, with 10 O positions and 8 C positions
var oPath = []struct{ row, col int }{
	{0, 0}, {1, 0}, {2, 0}, {3, 0}, // left side, top to bottom
	{3, 1}, {2, 1}, {1, 1}, {0, 1}, // right side, bottom to top
	{0, 0}, {0, 1}, // top left, top right (for smoothness)
}
var cPath = []struct{ row, col int }{
	{0, 0}, {1, 0}, {2, 0}, {3, 0}, // left side, top to bottom
	{3, 1}, {2, 1}, // bottom right, mid right (open top)
	{0, 1}, // top right (open)
	{0, 0}, // loop
}

// Easing for morphing
func easeInOutSine(t float64) float64 {
	return -(math.Cos(math.Pi*t) - 1) / 2
}

// ViewO renders the O morphing into C with a squeeze/open effect
func (s *OpenCodeSpinner) ViewO() string {
	t := theme.CurrentTheme()
	grid := [4][2]int{} // fade level per cell

	// O→C morph progress: 0.0–1.0
	var morph float64
	if s.progress < 1.0 {
		morph = easeInOutSine(s.progress)
	} else {
		morph = 1 - easeInOutSine(s.progress-1)
	}

	// Squeeze O horizontally and open right side
	for row := 0; row < 4; row++ {
		// Left column always present, fades with morph
		fade := s.trailLength
		if morph > 0.7 {
			fade = int(float64(s.trailLength) * (1.0 - (morph-0.7)/0.3))
		}
		grid[row][0] = fade
		// Right column: fades out as O opens
		if morph < 0.5 {
			grid[row][1] = int(float64(s.trailLength) * (1.0 - morph*2))
		} else {
			grid[row][1] = 0
		}
	}
	// Top and bottom right corners fade in as C
	if morph > 0.5 {
		grid[0][1] = int(float64(s.trailLength) * (morph - 0.5) * 2)
		grid[3][1] = int(float64(s.trailLength) * (morph - 0.5) * 2)
	}
	// Add a little bounce at the end of morph
	if morph > 0.95 {
		grid[1][1] = int(float64(s.trailLength) * (morph - 0.95) * 20)
		grid[2][1] = int(float64(s.trailLength) * (morph - 0.95) * 20)
	}

	// Render
	var b strings.Builder
	for row := 0; row < 4; row++ {
		for col := 0; col < 2; col++ {
			fade := grid[row][col]
			style := fadeStyle(t, fade, s.trailLength)
			b.WriteString(style.Render("██"))
		}
		if row < 3 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// ViewC renders the C morphing from O with a grow/bounce effect
func (s *OpenCodeSpinner) ViewC() string {
	t := theme.CurrentTheme()
	grid := [4][2]int{} // fade level per cell

	// O→C morph progress: 0.0–1.0
	var morph float64
	if s.progress < 1.0 {
		morph = easeInOutSine(s.progress)
	} else {
		morph = 1 - easeInOutSine(s.progress-1)
	}

	// C grows from the O opening, with a bounce
	for row := 0; row < 4; row++ {
		// Left column: fade in as morph progresses
		if morph > 0.5 {
			grid[row][0] = int(float64(s.trailLength) * (morph - 0.5) * 2)
		}
		// Right column: only top and bottom, fade in with morph
		if row == 0 || row == 3 {
			if morph > 0.7 {
				grid[row][1] = int(float64(s.trailLength) * (morph - 0.7) / 0.3)
			}
		}
	}
	// Add a snap/overshoot at the end
	if morph > 0.95 {
		grid[1][1] = int(float64(s.trailLength) * (morph - 0.95) * 20)
		grid[2][1] = int(float64(s.trailLength) * (morph - 0.95) * 20)
	}

	// Render
	var b strings.Builder
	for row := 0; row < 4; row++ {
		for col := 0; col < 2; col++ {
			fade := grid[row][col]
			style := fadeStyle(t, fade, s.trailLength)
			b.WriteString(style.Render("██"))
		}
		if row < 3 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// fadeStyle returns a lipgloss style for a given fade level
func fadeStyle(t theme.Theme, fade, maxFade int) lipgloss.Style {
	switch {
	case fade >= maxFade:
		return lipgloss.NewStyle().Foreground(t.Primary()).Bold(true)
	case fade >= maxFade-1:
		return lipgloss.NewStyle().Foreground(t.Primary()).Faint(true)
	case fade >= maxFade-2:
		return lipgloss.NewStyle().Foreground(t.TextMuted())
	case fade >= 1:
		return lipgloss.NewStyle().Foreground(t.TextMuted()).Faint(true)
	default:
		return lipgloss.NewStyle().Foreground(t.Background())
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
