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

// AnimationPhase represents which letter is currently animating
type AnimationPhase int

const (
	PhaseOAnimating AnimationPhase = iota
	PhaseOSleeping
	PhaseCAnimating
	PhaseCSleeping
)

// OpenCodeSpinner represents the animated O and C letter spinner
type OpenCodeSpinner struct {
	currentOFrame   int
	currentCFrame   int
	oFadeFrames     []int // tracks fade intensity for each position in O
	cFadeFrames     []int // tracks fade intensity for each position in C
	isAnimating     bool
	interval        time.Duration
	totalFrames     int // total frames elapsed
	currentPhase    AnimationPhase
	phaseFrameCount int // frames within current phase
	oLoopFrames     int // number of frames for O to complete a loop
	cLoopFrames     int // number of frames for C to complete a loop
	sleepFrames     int // number of frames to sleep between animations
}

// New creates a new OpenCode spinner
func New() *OpenCodeSpinner {
	return &OpenCodeSpinner{
		currentOFrame:   0,
		currentCFrame:   0,
		oFadeFrames:     make([]int, 10), // 10 positions for the O letter (4-tall rectangle)
		cFadeFrames:     make([]int, 8),  // 8 positions for the C letter (4-tall rectangle)
		isAnimating:     false,
		interval:        120 * time.Millisecond,
		totalFrames:     0,
		currentPhase:    PhaseOAnimating,
		phaseFrameCount: 0,
		oLoopFrames:     10, // O has 10 positions to complete
		cLoopFrames:     8,  // C has 8 positions to complete
		sleepFrames:     8,  // sleep for 8 frames between animations
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

		s.totalFrames++
		s.phaseFrameCount++

		// Handle different animation phases
		switch s.currentPhase {
		case PhaseOAnimating:
			// Update fade for all O positions
			for i := range s.oFadeFrames {
				if s.oFadeFrames[i] > 0 {
					s.oFadeFrames[i]--
				}
			}

			// Set current O position to max fade
			s.oFadeFrames[s.currentOFrame] = 4

			// Move O to next frame (counter-clockwise)
			s.currentOFrame = (s.currentOFrame - 1 + len(s.oFadeFrames)) % len(s.oFadeFrames)

			// Check if O completed its loop
			if s.phaseFrameCount >= s.oLoopFrames {
				s.currentPhase = PhaseOSleeping
				s.phaseFrameCount = 0
			}

		case PhaseOSleeping:
			// O is sleeping, decay its fade
			for i := range s.oFadeFrames {
				if s.oFadeFrames[i] > 0 {
					s.oFadeFrames[i]--
				}
			}

			// Sleep completed, start C animation
			if s.phaseFrameCount >= s.sleepFrames {
				s.currentPhase = PhaseCAnimating
				s.phaseFrameCount = 0
			}

		case PhaseCAnimating:
			// Update fade for all C positions
			for i := range s.cFadeFrames {
				if s.cFadeFrames[i] > 0 {
					s.cFadeFrames[i]--
				}
			}

			// Set current C position to max fade
			s.cFadeFrames[s.currentCFrame] = 4

			// Move C to next frame (clockwise)
			s.currentCFrame = (s.currentCFrame + 1) % len(s.cFadeFrames)

			// Check if C completed its loop
			if s.phaseFrameCount >= s.cLoopFrames {
				s.currentPhase = PhaseCSleeping
				s.phaseFrameCount = 0
			}

		case PhaseCSleeping:
			// C is sleeping, decay its fade
			for i := range s.cFadeFrames {
				if s.cFadeFrames[i] > 0 {
					s.cFadeFrames[i]--
				}
			}

			// Sleep completed, start O animation again
			if s.phaseFrameCount >= s.sleepFrames {
				s.currentPhase = PhaseOAnimating
				s.phaseFrameCount = 0
			}
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

// ViewO renders just the O letter animation
func (s *OpenCodeSpinner) ViewO() string {
	t := theme.CurrentTheme()

	// Define the O letter sprite positions (2x4 grid, counter-clockwise from top-left)
	// Based on the sprite image: O is 2 pixels wide, 4 pixels tall
	// Positions: 0=top-left, 1=left-top, 2=left-bottom, 3=bottom-left, 4=bottom-right, 5=right-bottom, 6=right-top, 7=top-right, 8=top-mid-left, 9=top-mid-right
	oPositions := [][]int{
		{0, 9}, // top row
		{8, 6}, // second row
		{1, 5}, // third row
		{2, 4}, // bottom row
	}

	var builder strings.Builder

	for row := 0; row < 4; row++ {
		for col := 0; col < 2; col++ {
			pos := oPositions[row][col]

			// Get fade level for this position
			fade := s.oFadeFrames[pos]

			// Create styled block based on fade level
			var style lipgloss.Style
			switch fade {
			case 4: // Brightest (current position)
				style = lipgloss.NewStyle().Foreground(t.Primary())
			case 3: // Fade level 1
				style = lipgloss.NewStyle().Foreground(t.Primary()).Faint(true)
			case 2: // Fade level 2
				style = lipgloss.NewStyle().Foreground(t.TextMuted())
			case 1: // Fade level 3
				style = lipgloss.NewStyle().Foreground(t.TextMuted()).Faint(true)
			default: // Invisible
				style = lipgloss.NewStyle().Foreground(t.Background())
			}

			// Use solid block character
			builder.WriteString(style.Render("██"))
		}
		if row < 3 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// ViewC renders just the C letter animation
func (s *OpenCodeSpinner) ViewC() string {
	t := theme.CurrentTheme()

	// Define the C letter sprite positions (2x4 grid)
	// Based on the sprite image: C is 2 pixels wide, 4 pixels tall, open on the right
	// Positions: 0=top-left, 1=left-top, 2=left-bottom, 3=bottom-left, 4=top-right (only top), 5=bottom-right (only bottom), 6=left-mid-top, 7=left-mid-bottom
	cPositions := [][]int{
		{0, 4},  // top row (top-left, top-right)
		{6, -1}, // second row (left only, C is open on right)
		{7, -1}, // third row (left only, C is open on right)
		{3, 5},  // bottom row (bottom-left, bottom-right)
	}

	var builder strings.Builder

	for row := 0; row < 4; row++ {
		for col := 0; col < 2; col++ {
			pos := cPositions[row][col]

			if pos == -1 {
				// Empty space (C is open on the right)
				builder.WriteString("  ")
			} else {
				// Get fade level for this position
				fade := s.cFadeFrames[pos]

				// Create styled block based on fade level
				var style lipgloss.Style
				switch fade {
				case 4: // Brightest (current position)
					style = lipgloss.NewStyle().Foreground(t.Primary())
				case 3: // Fade level 1
					style = lipgloss.NewStyle().Foreground(t.Primary()).Faint(true)
				case 2: // Fade level 2
					style = lipgloss.NewStyle().Foreground(t.TextMuted())
				case 1: // Fade level 3
					style = lipgloss.NewStyle().Foreground(t.TextMuted()).Faint(true)
				default: // Invisible
					style = lipgloss.NewStyle().Foreground(t.Background())
				}

				// Use solid block character
				builder.WriteString(style.Render("████"))
			}
		}
		if row < 3 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// View renders the current frame of the spinner (both O and C)
func (s *OpenCodeSpinner) View() string {
	oView := s.ViewO()
	cView := s.ViewC()

	return lipgloss.JoinHorizontal(lipgloss.Top, oView, "  ", cView)
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
