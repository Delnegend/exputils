package main

import (
	"context"
	"exputils/tasks"
	wexpmonitor "exputils/wexp_monitor"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type Button string

const (
	ButtonNone Button = "btn-none"

	EnablePollingBtn  Button = "enable-polling"
	DisablePollingBtn Button = "disable-polling"

	CancelTaskBtn Button = "cancel-task"
	StartTaskBtn  Button = "start-task"

	ArtefactBtn Button = "artefact"
	DjxlBtn     Button = "djxl"
	JxlBtn      Button = "jxl"
	LossyJxlBtn Button = "lossy-jxl"
	Par2Btn     Button = "par2"
)

type MainModel struct {
	spinner          spinner.Model
	lastViewPath     string
	lastViewPathChan chan string
	isPolling        bool
	pollTicker       *time.Ticker

	clicked Button
	hovered Button

	someTaskContext     context.Context
	someTaskRunning     bool
	someTaskRunningChan chan bool
}

type PathUpdateMsg struct{ path string }
type SomeTaskRunningMsg struct{ running bool }

func newMainModel() MainModel {
	return MainModel{
		spinner: spinner.New(func(m *spinner.Model) {
			m.Spinner = spinner.MiniDot
		}),
		lastViewPath:     "",
		lastViewPathChan: make(chan string),
		isPolling:        true,
		pollTicker:       time.NewTicker(500 * time.Millisecond),

		clicked: ButtonNone,
		hovered: ButtonNone,

		someTaskContext:     context.Background(),
		someTaskRunning:     false,
		someTaskRunningChan: make(chan bool),
	}
}

func (m MainModel) Init() tea.Cmd {
	go func() {
		for range m.pollTicker.C {
			newPath, err := wexpmonitor.GetLastViewedExplorerPath()
			if err == nil && newPath != m.lastViewPath {
				m.lastViewPathChan <- newPath
			}
		}
	}()

	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			return PathUpdateMsg{<-m.lastViewPathChan}
		},
		func() tea.Msg {
			return SomeTaskRunningMsg{<-m.someTaskRunningChan}
		},
	)
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case PathUpdateMsg:
		m.lastViewPath = msg.path
		return m, func() tea.Msg {
			return PathUpdateMsg{<-m.lastViewPathChan}
		}
	case SomeTaskRunningMsg:
		m.someTaskRunning = msg.running
		return m, func() tea.Msg {
			return SomeTaskRunningMsg{<-m.someTaskRunningChan}
		}
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionMotion { // aka hover
			buttonZones := map[Button]string{
				DisablePollingBtn: string(DisablePollingBtn),
				EnablePollingBtn:  string(EnablePollingBtn),
				CancelTaskBtn:     string(CancelTaskBtn),
				StartTaskBtn:      string(StartTaskBtn),
				ArtefactBtn:       string(ArtefactBtn),
				DjxlBtn:           string(DjxlBtn),
				JxlBtn:            string(JxlBtn),
				LossyJxlBtn:       string(LossyJxlBtn),
				Par2Btn:           string(Par2Btn),
			}

			m.hovered = ButtonNone // Default to no hover

		scoped:
			for button, zoneName := range buttonZones {
				if zone.Get(zoneName).InBounds(msg) {
					m.hovered = button
					break scoped
				}
			}
		}
		if msg.Action == tea.MouseActionRelease {
			m.clicked = ButtonNone
			return m, nil
		}
		// basically onClick in javascript at this point
		if msg.Button != tea.MouseButtonLeft {
			break
		}
		switch {
		case zone.Get(string(DisablePollingBtn)).InBounds(msg):
			if !m.isPolling {
				break
			}
			m.pollTicker.Stop()
			m.isPolling = false
			m.clicked = DisablePollingBtn
		case zone.Get(string(EnablePollingBtn)).InBounds(msg):
			if m.isPolling {
				break
			}
			m.pollTicker.Reset(500 * time.Millisecond)
			m.isPolling = true
			m.clicked = EnablePollingBtn
		case zone.Get(string(CancelTaskBtn)).InBounds(msg):
			if !m.someTaskRunning {
				break
			}
			m.someTaskContext.Done()
			m.someTaskContext = context.Background()
			m.someTaskRunning = false
			m.clicked = CancelTaskBtn
		case zone.Get(string(StartTaskBtn)).InBounds(msg):
			if m.someTaskRunning {
				break
			}
			go func() {
				if err := tasks.ExampleTask(m.someTaskContext); err != nil {
					fmt.Println(err)
				} else {
					m.someTaskContext = context.Background()
					m.someTaskRunningChan <- false
				}
			}()
			m.someTaskRunning = true
			m.clicked = StartTaskBtn
		default:
			m.clicked = ButtonNone
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "c":
			if !m.someTaskRunning {
				break
			}
			m.someTaskContext.Done()
			m.someTaskContext = context.Background()
			m.someTaskRunning = false
			m.clicked = CancelTaskBtn
		}
	}
	return m, nil
}

var (
	btnFrame = lipgloss.NewStyle().
			Width(24).
			Align(lipgloss.Center).
			Padding(0, 1).
			Border(lipgloss.NormalBorder())

	baseBtnStyle = btnFrame.
			Foreground(lipgloss.Color("#FFF7DB"))

	activeBtnStyle = btnFrame.
			Background(lipgloss.Color("#F25D94")).
			BorderBackground(lipgloss.Color("#F25D94")).
			Foreground(lipgloss.Color("#FFF7DB")).
			Bold(true)

	hoveredBtnStyle = btnFrame.
			Border(lipgloss.DoubleBorder()).
			Foreground(lipgloss.Color("#FFF7DB"))

	disabledBtnStyle = btnFrame.
				BorderForeground(lipgloss.Color("#525252")).
				Foreground(lipgloss.Color("#525252"))
)

func (m *MainModel) ButtonStyler(content string, kind Button, disabled bool) string {
	style := baseBtnStyle
	if disabled {
		style = disabledBtnStyle
	} else {
		if kind == m.hovered {
			style = hoveredBtnStyle
		}
		if kind == m.clicked {
			style = activeBtnStyle
		}
	}
	return zone.Mark(string(kind), style.Render(content))
}

func (m MainModel) View() string {
	divider := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666565")).
		SetString(fmt.Sprintf("<%s>", strings.Repeat("─", 82))).
		String()

	return zone.Scan(lipgloss.JoinVertical(
		lipgloss.Top,
		lipgloss.
			NewStyle().
			Foreground(lipgloss.Color("#949494")).
			PaddingLeft(2).
			Render(
				func() string {
					var sb strings.Builder
					if m.isPolling {
						sb.WriteString(m.spinner.View())
						sb.WriteString("  ")
					} else {
						sb.WriteString("🛑 ")
					}
					sb.WriteString(m.lastViewPath)
					return sb.String()
				}(),
			),
		divider,
		lipgloss.NewStyle().Margin(0, 0, 0, 2).Render(
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				m.ButtonStyler(
					"Disable Polling",
					DisablePollingBtn,
					!m.isPolling,
				),
				m.ButtonStyler(
					"Enable Polling",
					EnablePollingBtn,
					m.isPolling,
				),
				m.ButtonStyler(
					"Cancel Task",
					CancelTaskBtn,
					!m.someTaskRunning,
				),
			),
		),
		divider,
		lipgloss.NewStyle().Margin(0, 0, 0, 2).Render(
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				m.ButtonStyler(
					"Artefact",
					ArtefactBtn,
					m.someTaskRunning,
				),
				m.ButtonStyler(
					"All 2 Lossless JPEG-XL",
					JxlBtn,
					m.someTaskRunning,
				),
				m.ButtonStyler(
					"PNG 2 Lossy JPEG-XL",
					LossyJxlBtn,
					m.someTaskRunning,
				),
			),
		),
		lipgloss.NewStyle().Margin(0, 0, 0, 2).Render(
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				m.ButtonStyler(
					"JPEG-XL 2 PNG, JPG",
					DjxlBtn,
					m.someTaskRunning,
				),
				m.ButtonStyler(
					"PAR2 from 7z",
					Par2Btn,
					m.someTaskRunning,
				),
				m.ButtonStyler(
					"Start Task",
					StartTaskBtn,
					m.someTaskRunning,
				),
			),
		),
		divider,
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#949494")).
			PaddingLeft(2).
			Render("q, ctrl+c: Quit | c: Cancel Task"),
	))
}

func main() {
	zone.NewGlobal()
	defer zone.Close()

	p := tea.NewProgram(newMainModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
