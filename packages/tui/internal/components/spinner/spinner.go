package spinner

import (
	_ "embed"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/sst/opencode/internal/theme"
)

//go:embed frames.md
var framesData string

type TickMsg struct{ Time time.Time }

type OpenCodeSpinner struct {
	currentFrame int
	frames       []string
	interval     time.Duration
	isAnimating  bool
}

func New() *OpenCodeSpinner {
	return &OpenCodeSpinner{
		frames:   parseFrames(framesData),
		interval: 90 * time.Millisecond,
	}
}

func (s *OpenCodeSpinner) Init() tea.Cmd {
	s.isAnimating = true
	return s.tick()
}

func (s *OpenCodeSpinner) Update(msg tea.Msg) (*OpenCodeSpinner, tea.Cmd) {
	if _, ok := msg.(TickMsg); ok && s.isAnimating {
		s.currentFrame = (s.currentFrame + 1) % len(s.frames)
		return s, s.tick()
	}
	return s, nil
}

func (s *OpenCodeSpinner) View() string {
	if len(s.frames) == 0 {
		return ""
	}
	return s.render(s.frames[s.currentFrame])
}

func (s *OpenCodeSpinner) ViewOCOnly() string {
	return s.View()
}

func (s *OpenCodeSpinner) tick() tea.Cmd {
	return tea.Tick(s.interval, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}

func (s *OpenCodeSpinner) render(frame string) string {
	style := lipgloss.NewStyle().Foreground(theme.CurrentTheme().Primary())
	var result strings.Builder

	for i, line := range strings.Split(frame, "\n") {
		for _, char := range line {
			switch char {
			case 'x':
				result.WriteString(style.Render("█"))
			case 'w':
				result.WriteString(style.Render("▌"))
			case 'c':
				result.WriteString(style.Render("▐"))
			case 'z':
				result.WriteString(style.Render("▀"))
			case 's':
				result.WriteString(style.Render("▄"))
			case '.':
				result.WriteString(" ")
			default:
				result.WriteString(string(char))
			}
		}
		if i < len(strings.Split(frame, "\n"))-1 {
			result.WriteString("\n")
		}
	}
	return result.String()
}

func parseFrames(data string) []string {
	// Remove /* */ comments
	commentRegex := regexp.MustCompile(`/\*.*?\*/`)
	data = commentRegex.ReplaceAllString(data, "")

	var frames []string
	var frameLines []string

	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "---" {
			if len(frameLines) > 0 {
				frames = append(frames, strings.Join(frameLines, "\n"))
				frameLines = frameLines[:0]
			}
		} else if line != "" {
			frameLines = append(frameLines, line)
		}
	}

	if len(frameLines) > 0 {
		frames = append(frames, strings.Join(frameLines, "\n"))
	}
	return frames
}
