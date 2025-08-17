package spinner

import (
	_ "embed"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/sst/opencode/internal/theme"
)

//go:embed frames.md
var framesData string

type TickMsg struct {
	Time time.Time
}

type OpenCodeSpinner struct {
	currentFrame int
	frames       []string
	interval     time.Duration
	isAnimating  bool
}

func New() *OpenCodeSpinner {
	spinner := &OpenCodeSpinner{
		currentFrame: 0,
		interval:     90 * time.Millisecond, // 10fps
		isAnimating:  false,
	}
	spinner.frames = ParseFramesMD(framesData)
	return spinner
}

func (s *OpenCodeSpinner) Init() tea.Cmd {
	s.isAnimating = true
	return s.tick()
}

func (s *OpenCodeSpinner) tick() tea.Cmd {
	return tea.Tick(s.interval, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}

func (s *OpenCodeSpinner) Update(msg tea.Msg) (*OpenCodeSpinner, tea.Cmd) {
	switch msg.(type) {
	case TickMsg:
		if !s.isAnimating {
			return s, nil
		}
		s.currentFrame = (s.currentFrame + 1) % len(s.frames)
		return s, s.tick()
	}
	return s, nil
}

func (s *OpenCodeSpinner) View() string {
	if len(s.frames) == 0 {
		return ""
	}
	return s.renderFrame(s.frames[s.currentFrame])
}

func (s *OpenCodeSpinner) ViewOCOnly() string {
	return s.View()
}

func (s *OpenCodeSpinner) renderFrame(frame string) string {
	t := theme.CurrentTheme()
	var builder strings.Builder
	lines := strings.Split(frame, "\n")

	for i, line := range lines {
		for _, char := range line {
			switch char {
			case 'x':
				style := lipgloss.NewStyle().Foreground(t.Primary())
				builder.WriteString(style.Render("█"))
			case 'w':
				style := lipgloss.NewStyle().Foreground(t.Primary())
				builder.WriteString(style.Render("▌"))
			case 'c':
				style := lipgloss.NewStyle().Foreground(t.Primary())
				builder.WriteString(style.Render("▐"))
			case 'z':
				style := lipgloss.NewStyle().Foreground(t.Primary())
				builder.WriteString(style.Render("▀"))
			case 's':
				style := lipgloss.NewStyle().Foreground(t.Primary())
				builder.WriteString(style.Render("▄"))
			case '.':
				builder.WriteString(" ")
			default:
				builder.WriteString(string(char))
			}
		}
		if i < len(lines)-1 {
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func ParseFramesMD(md string) []string {
	var frames []string
	var frameLines []string
	for _, line := range strings.Split(md, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "---" {
			if len(frameLines) > 0 {
				frames = append(frames, strings.Join(frameLines, "\n"))
				frameLines = frameLines[:0]
			}
			continue
		}
		if trim == "" {
			continue
		}
		frameLines = append(frameLines, line)
	}
	if len(frameLines) > 0 {
		frames = append(frames, strings.Join(frameLines, "\n"))
	}
	return frames
}
