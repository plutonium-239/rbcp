package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
var fixedWidth = lipgloss.NewStyle().Width(8)
var impStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5956E0")).Bold(true)
var pathStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ADBDFF")).Italic(true)
var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#DA4167")).Bold(true)

type UpdateMsg struct {
	file string
	// incr bool
	bytes int64
}

type tickMsg struct{}

func finalPause() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

type model struct {
	progress    progress.Model
	percent     float64
	currentFile string

	totalBytes  int64
	totalFiles  int
	copiedBytes int64
	copiedFiles int

	copyFinished bool
	stats        *RobocopyStats
	numTimes     int
	numMsgs      int

	totalWidth int
}

func (m model) Init() tea.Cmd {
	// log.Infof("To complete: %v bytes", m.totalBytes)
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// case tea.KeyMsg:
	// 	return m, tea.Quit

	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - 1*2 - 8*2
		m.totalWidth = msg.Width - 1*2
		return m, nil

	case UpdateMsg:
		cmd := m.UpdateProcessor(msg)
		return m, cmd

	case tickMsg:
		// if m.progress.Percent() < 1.0 {
		// 	// log.Warnf("not completed yet, will extend")
		// 	return m, finalPause()
		// }
		m.copyFinished = true
		return m, tea.Quit

	// FrameMsg is sent when the progress bar wants to animate itself
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		m.numTimes += 1
		return m, cmd

	default:
		return m, nil
	}
}

func (m model) View() string {
	// summary := ""
	currentFile := helpStyle.Render("Currently copying ", m.currentFile)
	if m.copyFinished {
		// summary = fmt.Sprintf("\nProcessed %v msgs and animated %v times\n\n", m.numMsgs, m.numTimes)
		currentFile = helpStyle.Render("Copying completed")
	}
	files := fmt.Sprintf("%v/%v", m.copiedFiles, m.totalFiles)
	bytes := fixedWidth.Render(formatByteValue(m.copiedBytes)) + "/" + fixedWidth.Render(formatByteValue(m.totalBytes))
	// return bytes + " " + m.progress.View() + " \n" +
	return bytes + " " + m.progress.ViewAs(m.percent) + " \n" +
		" " + JustifyText(m.totalWidth, currentFile, files) + " \n"
	// summary + "\n"
}

func (m *model) UpdateProcessor(msg UpdateMsg) tea.Cmd {
	m.numMsgs += 1
	if msg.file != "" {
		m.currentFile = msg.file
		m.copiedFiles += 1
	}

	m.copiedBytes += msg.bytes
	// log.Infof("Received UpdateMsg %v, Copied = %v bytes", msg, m.copiedBytes)
	m.percent = float64(m.copiedBytes) / float64(m.totalBytes)
	// cmds := []tea.Cmd{m.progress.SetPercent(percent)}
	// cmds := []tea.Cmd{}
	var cmd tea.Cmd
	if m.percent >= 1.0 {
		m.copyFinished = true
		cmd = finalPause()
		// cmds = append(cmds, finalPause())
	}
	return cmd
}

func JustifyText(width int, texts ...string) string {
	totalLen := 0
	for _, t := range texts {
		totalLen += lipgloss.Width(t)
	}
	// logger.Debug("dims are", "width", width, "totalLen", totalLen)
	if totalLen > width {
		log.Warn("Content too small for terminal width", "width", width, "totalLen", totalLen)
		return lipgloss.JoinHorizontal(lipgloss.Left, texts...)
	}
	if len(texts) == 1 {
		return texts[0] + strings.Repeat(" ", (width-totalLen))
	}
	gapSize := (width - totalLen) / (len(texts) - 1)
	remainder := strings.Repeat(" ", (width-totalLen)%(len(texts)-1))
	texts[0] += remainder
	// logger.Debug("laying out", "len(texts)", len(texts), "gapSize", gapSize)
	return strings.Join(texts, strings.Repeat(" ", gapSize))
}
