package main

import (
	"context"
	"exputils/tasks"
	wexpmonitor "exputils/wexp_monitor"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type Button struct{ ID, Label string }

var (
	NoneButton = Button{"", ""}

	EnablePollingButton  = Button{"enable-polling", "Enable Polling"}
	DisablePollingButton = Button{"disable-polling", "Disable Polling"}
	CancelTaskButton     = Button{"cancel-task", "Cancel Task"}

	StartTaskButton = Button{"start-task", "Start Task"}
	ArtefactButton  = Button{"artefact", "Artefact"}
	DjxlButton      = Button{"djxl", "JPEG-XL 2 PNG, JPG"}
	JxlButton       = Button{"jxl", "All 2 Lossless JPEG-XL"}
	LossyJxlButton  = Button{"lossy-jxl", "PNG 2 Lossy JPEG-XL"}
	Par2Button      = Button{"par2", "PAR2 from 7z"}

	someTaskRunningChan = make(chan bool)
	warnChan            = make(chan error)
	setProgressChan     = make(chan float64)

	pollLastViewPathTicker = time.NewTicker(500 * time.Millisecond)
	lastViewPathChan       = make(chan string)

	taskCtx, taskCancel = context.WithCancel(context.Background())
)

type MainModel struct {
	lastViewPath    string
	isPolling       bool
	clicked         *Button
	hovered         *Button
	someTaskRunning bool

	spinner  spinner.Model
	progress progress.Model

	accumulatedWarns []error
}

func NewMainModel() MainModel {
	return MainModel{
		lastViewPath:    "",
		isPolling:       true,
		clicked:         &NoneButton,
		hovered:         &NoneButton,
		someTaskRunning: false,

		spinner: spinner.New(func(m *spinner.Model) {
			m.Spinner = spinner.MiniDot
		}),
		progress: progress.New(progress.WithDefaultGradient(), progress.WithWidth(80)),

		accumulatedWarns: []error{},
	}
}

func (m *MainModel) SpawnTask(fn func(ctx context.Context)) {
	if m.someTaskRunning {
		return
	}
	m.accumulatedWarns = []error{}
	taskCancel()
	taskCtx, taskCancel = context.WithCancel(context.Background())
	go func() {
		someTaskRunningChan <- true
		fn(taskCtx)
		someTaskRunningChan <- false
		setProgressChan <- 0
	}()
}

type NewLastViewPathMsg struct{ path string }
type SomeTaskRunningMsg struct{ running bool }
type SetProgressPercentMsg struct{ value float64 }
type WarnMsg struct{ warn error }

func FetchLatestViewPath() tea.Msg     { return NewLastViewPathMsg{<-lastViewPathChan} }
func FetchSomeTaskRunning() tea.Msg    { return SomeTaskRunningMsg{<-someTaskRunningChan} }
func FetchSetProgressPercent() tea.Msg { return SetProgressPercentMsg{<-setProgressChan} }
func FetchWarn() tea.Msg               { return WarnMsg{<-warnChan} }

func (m MainModel) Init() tea.Cmd {
	go func() {
		for range pollLastViewPathTicker.C {
			newPath, err := wexpmonitor.GetLastViewedExplorerPath()
			if err == nil && newPath != "" && newPath != m.lastViewPath {
				lastViewPathChan <- newPath
			}
		}
	}()

	return tea.Batch(
		m.spinner.Tick,
		FetchLatestViewPath,
		FetchSomeTaskRunning,
		FetchSetProgressPercent,
		FetchWarn,
	)
}

// WARNING: DO NOT EXEC BLOCKING EVENTS IN THIS FUNCTION
// including sending messages to channels, spawn a goroutine
// and send messages there instead
func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	case SetProgressPercentMsg:
		cmd := m.progress.SetPercent(msg.value)
		return m, tea.Batch(cmd, FetchSetProgressPercent)
	case NewLastViewPathMsg:
		m.lastViewPath = msg.path
		return m, FetchLatestViewPath
	case SomeTaskRunningMsg:
		m.someTaskRunning = msg.running
		if !m.someTaskRunning {
			go func() { setProgressChan <- 0 }()
		}
		return m, FetchSomeTaskRunning
	case WarnMsg:
		m.accumulatedWarns = append(m.accumulatedWarns, msg.warn)
		return m, FetchWarn
	case tea.MouseMsg:
		m.hovered = &NoneButton
		if msg.Action == tea.MouseActionMotion { // aka hover
			switch {
			case zone.Get(EnablePollingButton.ID).InBounds(msg):
				m.hovered = &EnablePollingButton
			case zone.Get(DisablePollingButton.ID).InBounds(msg):
				m.hovered = &DisablePollingButton
			case zone.Get(CancelTaskButton.ID).InBounds(msg):
				m.hovered = &CancelTaskButton
			case zone.Get(StartTaskButton.ID).InBounds(msg):
				m.hovered = &StartTaskButton
			case zone.Get(ArtefactButton.ID).InBounds(msg):
				m.hovered = &ArtefactButton
			case zone.Get(DjxlButton.ID).InBounds(msg):
				m.hovered = &DjxlButton
			case zone.Get(JxlButton.ID).InBounds(msg):
				m.hovered = &JxlButton
			case zone.Get(LossyJxlButton.ID).InBounds(msg):
				m.hovered = &LossyJxlButton
			case zone.Get(Par2Button.ID).InBounds(msg):
				m.hovered = &Par2Button
			}
		}
		if msg.Button == tea.MouseButtonLeft { // aka onClick
			m.clicked = &NoneButton
			switch {
			case zone.Get(EnablePollingButton.ID).InBounds(msg):
				if !m.isPolling {
					pollLastViewPathTicker.Reset(500 * time.Millisecond)
					m.isPolling = true
					m.clicked = &EnablePollingButton
				}
			case zone.Get(DisablePollingButton.ID).InBounds(msg):
				if m.isPolling {
					pollLastViewPathTicker.Stop()
					m.isPolling = false
					m.clicked = &DisablePollingButton
				}
			case zone.Get(CancelTaskButton.ID).InBounds(msg):
				taskCancel()
				go func() { setProgressChan <- 0 }()
				m.clicked = &CancelTaskButton
			case zone.Get(StartTaskButton.ID).InBounds(msg):
				if !m.someTaskRunning {
					m.clicked = &StartTaskButton
					m.SpawnTask(func(ctx context.Context) {
						tasks.ExampleTask(ctx, setProgressChan, warnChan)
					})
				}
			case zone.Get(ArtefactButton.ID).InBounds(msg):
				m.clicked = &ArtefactButton
			case zone.Get(DjxlButton.ID).InBounds(msg):
				if !m.someTaskRunning {
					m.clicked = &DjxlButton
					m.SpawnTask(func(ctx context.Context) {
						tasks.DjxlToJpgPng(ctx, m.lastViewPath, 1, setProgressChan, warnChan)
					})
				}
			case zone.Get(JxlButton.ID).InBounds(msg):
				m.clicked = &JxlButton
			case zone.Get(LossyJxlButton.ID).InBounds(msg):
				m.clicked = &LossyJxlButton
			case zone.Get(Par2Button.ID).InBounds(msg):
				m.clicked = &Par2Button
			}
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			taskCancel()
			return m, tea.Quit
		case "c":
			if m.someTaskRunning {
				taskCancel()
				go func() {
					setProgressChan <- 0
					someTaskRunningChan <- false
				}()
			}
		}
	}
	return m, nil
}

func (m MainModel) View() string {
	divider := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666565")).
		SetString(fmt.Sprintf("<%s>", strings.Repeat("─", 82))).
		String()

	btnStyle := func(b *Button, disabled bool) string {
		btnFrame := lipgloss.NewStyle().
			Width(24).
			Align(lipgloss.Center).
			Padding(0, 1).
			Border(lipgloss.NormalBorder())

		baseBtnStyle := btnFrame.
			Foreground(lipgloss.Color("#FFF7DB"))

		style := baseBtnStyle
		if disabled {
			style = btnFrame.
				BorderForeground(lipgloss.Color("#525252")).
				Foreground(lipgloss.Color("#525252"))
		} else {
			if *m.hovered == *b {
				style = btnFrame.
					Border(lipgloss.DoubleBorder()).
					Foreground(lipgloss.Color("#FFF7DB"))
			}
			if *m.clicked == *b {
				style = btnFrame.
					Background(lipgloss.Color("#F25D94")).
					Foreground(lipgloss.Color("#FFF7DB")).
					Bold(true)
			}
		}
		return zone.Mark(b.ID, style.Render(b.Label))
	}

	return zone.Scan(lipgloss.JoinVertical(
		lipgloss.Top,
		lipgloss.
			NewStyle().
			Foreground(lipgloss.Color("#949494")).
			PaddingLeft(2).
			Render(func() string {
				if m.isPolling {
					return m.spinner.View() + "  " + m.lastViewPath
				} else {
					return "🛑 " + m.lastViewPath
				}
			}()),
		divider,
		lipgloss.NewStyle().Margin(0, 0, 0, 2).Render(lipgloss.JoinHorizontal(
			lipgloss.Top,
			btnStyle(&DisablePollingButton, !m.isPolling),
			btnStyle(&EnablePollingButton, m.isPolling),
			btnStyle(&CancelTaskButton, !m.someTaskRunning),
		)),
		divider,
		lipgloss.NewStyle().Margin(0, 0, 0, 2).Render(lipgloss.JoinHorizontal(
			lipgloss.Top,
			btnStyle(&ArtefactButton, m.someTaskRunning),
			btnStyle(&JxlButton, m.someTaskRunning),
			btnStyle(&LossyJxlButton, m.someTaskRunning),
		)),
		lipgloss.NewStyle().Margin(0, 0, 0, 2).Render(lipgloss.JoinHorizontal(
			lipgloss.Top,
			btnStyle(&DjxlButton, m.someTaskRunning),
			btnStyle(&Par2Button, m.someTaskRunning),
			btnStyle(&StartTaskButton, m.someTaskRunning),
		)),
		divider,
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#949494")).
			PaddingLeft(2).
			Render("q, ctrl+c: Quit | c: Cancel Task"),
		divider,
		"  "+m.progress.View(),
		divider,
		func() string {
			var sb strings.Builder
			for _, warn := range m.accumulatedWarns {
				if warn != nil {
					sb.WriteString("- " + warn.Error() + "\n")
				}
			}
			return sb.String()
		}(),
	))
}

func main() {
	zone.NewGlobal()
	defer zone.Close()

	p := tea.NewProgram(NewMainModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
