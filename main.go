package main

import (
	"context"
	"exputils/tasks"
	wexpmonitor "exputils/wexp_monitor"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type Button struct{ ID, Label string }

var (
	NoneButton = Button{"", ""}

	EnablePollingButton  = Button{"enable-polling", "Polling ON"}
	DisablePollingButton = Button{"disable-polling", "Polling OFF"}
	CancelTaskButton     = Button{"cancel-task", "Cancel Task"}

	StartTaskButton    = Button{"start-task", "Demo Task"}
	ArtefactButton     = Button{"artefact", "Artefact"}
	DjxlButton         = Button{"djxl", "DJXL"}
	CjxlLosslessButton = Button{"jxl", "Lossless JXL"}
	CjxlLossyButton    = Button{"lossy-jxl", "Lossy JXL"}
	Par2Button         = Button{"par2", "PAR2"}

	isPollingChan       = make(chan bool)
	someTaskRunningChan = make(chan *Button)
	warnChan            = make(chan error)
	setProgressChan     = make(chan float64)

	pollLastViewPathTicker = time.NewTicker(500 * time.Millisecond)
	lastViewPathChan       = make(chan string)

	taskCtx, taskCancel = context.WithCancel(context.Background())
)

type MainModel struct {
	lastViewPath    string
	isPolling       bool
	hovered         *Button
	someTaskRunning *Button

	spinner          spinner.Model
	progress         progress.Model
	warnViewport     viewport.Model
	accumulatedWarns []error
}

func NewMainModel() MainModel {
	return MainModel{
		lastViewPath:    "",
		isPolling:       true,
		hovered:         &NoneButton,
		someTaskRunning: &NoneButton,

		spinner:          spinner.New(func(m *spinner.Model) { m.Spinner = spinner.MiniDot }),
		progress:         progress.New(progress.WithDefaultGradient(), progress.WithWidth(60)),
		warnViewport:     viewport.New(60, 16),
		accumulatedWarns: []error{},
	}
}

func (m *MainModel) SpawnTask(fn func(
	ctx context.Context,
	sendWarning func(error),
	updateProgressBase func(func() float64) func(),
)) {
	if m.someTaskRunning == &NoneButton {
		return
	}
	m.accumulatedWarns = []error{}
	go func() { warnChan <- nil }()

	taskCtx, taskCancel = context.WithCancel(context.Background())
	go func(taskCtx context.Context) {
		isPollingChan <- false
		fn(
			taskCtx,
			func(warn error) {
				go func() { warnChan <- warn }()
			},
			func(f func() float64) func() {
				return func() {
					go func() { setProgressChan <- f() }()
				}
			},
		)
		isPollingChan <- true
		someTaskRunningChan <- &NoneButton
		setProgressChan <- 0
	}(taskCtx)
}

type NewLastViewPathMsg struct{ path string }
type SomeTaskRunningMsg struct{ running *Button }
type SetProgressPercentMsg struct{ value float64 }
type WarnMsg struct{ warn error }
type IsPollingMsg struct{ polling bool }

func FetchLatestViewPath() tea.Msg     { return NewLastViewPathMsg{<-lastViewPathChan} }
func FetchSomeTaskRunning() tea.Msg    { return SomeTaskRunningMsg{<-someTaskRunningChan} }
func FetchSetProgressPercent() tea.Msg { return SetProgressPercentMsg{<-setProgressChan} }
func FetchWarn() tea.Msg               { return WarnMsg{<-warnChan} }
func FetchIsPolling() tea.Msg          { return IsPollingMsg{<-isPollingChan} }

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
		FetchIsPolling,
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
		return m, FetchSomeTaskRunning

	case WarnMsg:
		m.accumulatedWarns = append(m.accumulatedWarns, msg.warn)
		m.warnViewport.SetContent(func() string {
			var sb strings.Builder
			for _, warn := range m.accumulatedWarns {
				if warn == nil {
					continue
				}
				sb.WriteString("- " + warn.Error() + "\n")
			}
			return sb.String()
		}())
		return m, FetchWarn

	case IsPollingMsg:
		m.isPolling = msg.polling
		if m.isPolling {
			pollLastViewPathTicker.Reset(500 * time.Millisecond)
		} else {
			pollLastViewPathTicker.Stop()
		}
		return m, FetchIsPolling

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
			case zone.Get(CjxlLosslessButton.ID).InBounds(msg):
				m.hovered = &CjxlLosslessButton
			case zone.Get(CjxlLossyButton.ID).InBounds(msg):
				m.hovered = &CjxlLossyButton
			case zone.Get(Par2Button.ID).InBounds(msg):
				m.hovered = &Par2Button
			}
		}

		if msg.Button != tea.MouseButtonLeft {
			break
		}

		switch { // aka onClick
		case zone.Get(EnablePollingButton.ID).InBounds(msg):
			go func() { isPollingChan <- true }()
		case zone.Get(DisablePollingButton.ID).InBounds(msg):
			go func() { isPollingChan <- false }()
		case zone.Get(CancelTaskButton.ID).InBounds(msg):
			taskCancel()
			go func() { setProgressChan <- 0 }()
		case zone.Get(StartTaskButton.ID).InBounds(msg):
			go func() { someTaskRunningChan <- &StartTaskButton }()
			m.SpawnTask(func(ctx context.Context, sendWarning func(error), updateProgressBase func(func() float64) func()) {
				tasks.ExampleTask(ctx, updateProgressBase, sendWarning)
			})
		case zone.Get(ArtefactButton.ID).InBounds(msg):
			go func() { someTaskRunningChan <- &ArtefactButton }()
			m.SpawnTask(func(ctx context.Context, sendWarning func(error), updateProgressBase func(func() float64) func()) {
				tasks.Artefact(ctx, m.lastViewPath, 3, updateProgressBase, sendWarning)
			})
		case zone.Get(DjxlButton.ID).InBounds(msg):
			go func() { someTaskRunningChan <- &DjxlButton }()
			m.SpawnTask(func(ctx context.Context, sendWarning func(error), updateProgressBase func(func() float64) func()) {
				tasks.Djxl(ctx, m.lastViewPath, 1, updateProgressBase, sendWarning)
			})
		case zone.Get(CjxlLosslessButton.ID).InBounds(msg):
			go func() { someTaskRunningChan <- &CjxlLosslessButton }()
			m.SpawnTask(func(ctx context.Context, sendWarning func(error), updateProgressBase func(func() float64) func()) {
				tasks.Cjxl(ctx, m.lastViewPath, 2, false, updateProgressBase, sendWarning)
			})
		case zone.Get(CjxlLossyButton.ID).InBounds(msg):
			go func() { someTaskRunningChan <- &CjxlLossyButton }()
			m.SpawnTask(func(ctx context.Context, sendWarning func(error), updateProgressBase func(func() float64) func()) {
				tasks.Cjxl(ctx, m.lastViewPath, 2, true, updateProgressBase, sendWarning)
			})
		case zone.Get(Par2Button.ID).InBounds(msg):
			go func() { someTaskRunningChan <- &Par2Button }()
			m.SpawnTask(func(ctx context.Context, sendWarning func(error), updateProgressBase func(func() float64) func()) {
				tasks.Par2(ctx, m.lastViewPath, 2, updateProgressBase, sendWarning)
			})
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			taskCancel()
			return m, tea.Quit
		case "c":
			if m.someTaskRunning == &NoneButton {
				break
			}
			taskCancel()
			go func() {
				setProgressChan <- 0
				someTaskRunningChan <- &NoneButton
			}()
		}
	}

	var viewportCmd tea.Cmd
	m.warnViewport, viewportCmd = m.warnViewport.Update(msg)

	return m, viewportCmd
}

func (m MainModel) View() string {
	divider := func(title string) string {
		var sb strings.Builder

		titleLength := len(title)
		leftPad := (62 - titleLength) / 2
		rightPad := 62 - titleLength - leftPad

		sb.WriteString("<" + strings.Repeat("─", leftPad))
		sb.WriteString(title)
		sb.WriteString(strings.Repeat("─", rightPad) + ">")

		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666565")).
			SetString(sb.String()).
			String()
	}

	btnStyle := func(b *Button, disabled bool) string {
		btnFrame := lipgloss.NewStyle().
			Width(18).
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
		} else if *m.hovered == *b {
			style = btnFrame.
				Border(lipgloss.DoubleBorder()).
				Foreground(lipgloss.Color("#FFF7DB"))
		}
		return zone.Mark(b.ID, style.Render(func() string {
			var sb strings.Builder
			if m.someTaskRunning == b {
				sb.WriteString(m.spinner.View())
				sb.WriteString(" ")
			}
			sb.WriteString(b.Label)
			return sb.String()
		}()))
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
		lipgloss.NewStyle().Margin(0, 0, 0, 2).Render(lipgloss.JoinHorizontal(
			lipgloss.Top,
			btnStyle(&DisablePollingButton, !m.isPolling),
			btnStyle(&EnablePollingButton, m.isPolling),
			btnStyle(&CancelTaskButton, m.someTaskRunning == &NoneButton),
		)),
		divider("Tasks"),
		lipgloss.NewStyle().Margin(0, 0, 0, 2).Render(lipgloss.JoinHorizontal(
			lipgloss.Top,
			btnStyle(&ArtefactButton, m.someTaskRunning != &NoneButton),
			btnStyle(&Par2Button, m.someTaskRunning != &NoneButton),
			btnStyle(&StartTaskButton, m.someTaskRunning != &NoneButton),
		)),
		lipgloss.NewStyle().Margin(0, 0, 0, 2).Render(lipgloss.JoinHorizontal(
			lipgloss.Top,
			btnStyle(&CjxlLosslessButton, m.someTaskRunning != &NoneButton),
			btnStyle(&CjxlLossyButton, m.someTaskRunning != &NoneButton),
			btnStyle(&DjxlButton, m.someTaskRunning != &NoneButton),
		)),
		divider("Progress"),
		"  "+m.progress.View(),
		divider(fmt.Sprintf("Warnings | %3.f%%", m.warnViewport.ScrollPercent()*100)),
		m.warnViewport.View(),
	))
}

func main() {
	zone.NewGlobal()
	defer zone.Close()

	exec.Command("mode", "con:", "cols=64", "lines=30").Run()

	p := tea.NewProgram(NewMainModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
