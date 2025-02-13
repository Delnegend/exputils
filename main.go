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

var (
	theSpinner = spinner.New(func(m *spinner.Model) {
		m.Spinner = spinner.MiniDot
	})
	lastViewPath = ""
	isPolling    = true
	pollTicker   = time.NewTicker(500 * time.Millisecond)

	clicked = ButtonNone
	hovered = ButtonNone

	taskCtx     context.Context
	taskCancel  context.CancelFunc
	taskRunning = false

	forceRerenderChan = make(chan struct{})
)

func init() {
	taskCtx, taskCancel = context.WithCancel(context.Background())
}

type MainModel struct{}

type ForceRerenderMsg struct{}

func (m MainModel) Init() tea.Cmd {
	go func() {
		for range pollTicker.C {
			newPath, err := wexpmonitor.GetLastViewedExplorerPath()
			if err == nil && newPath != "" && newPath != lastViewPath {
				lastViewPath = newPath
				forceRerenderChan <- struct{}{}
			}
		}
	}()

	return tea.Batch(
		theSpinner.Tick,
		func() tea.Msg {
			return ForceRerenderMsg{}
		},
	)
}

func resetRunningTask() {
	if !taskRunning {
		return
	}
	taskCancel()
	taskRunning = false
	forceRerenderChan <- struct{}{}
}

func spawnTask(fn func(context.Context) error) {
	if taskRunning {
		return
	}
	taskRunning = true
	taskCtx, taskCancel = context.WithCancel(context.Background())

	forceRerenderChan <- struct{}{}
	go func() {
		if err := fn(taskCtx); err != nil {
			// do something
		}
		resetRunningTask()
	}()
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ForceRerenderMsg:
		return m, func() tea.Msg {
			<-forceRerenderChan
			return ForceRerenderMsg{}
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
			hovered = ButtonNone
		scoped:
			for button, zoneName := range buttonZones {
				if zone.Get(zoneName).InBounds(msg) {
					hovered = button
					break scoped
				}
			}
		}
		if msg.Action == tea.MouseActionRelease {
			clicked = ButtonNone
			return m, nil
		}
		// basically onClick in javascript at this point
		if msg.Button != tea.MouseButtonLeft {
			break
		}
		switch {
		case zone.Get(string(DisablePollingBtn)).InBounds(msg):
			if !isPolling {
				break
			}
			pollTicker.Stop()
			isPolling = false
			clicked = DisablePollingBtn
		case zone.Get(string(EnablePollingBtn)).InBounds(msg):
			if isPolling {
				break
			}
			pollTicker.Reset(500 * time.Millisecond)
			isPolling = true
			clicked = EnablePollingBtn
		case zone.Get(string(CancelTaskBtn)).InBounds(msg):
			resetRunningTask()
			clicked = CancelTaskBtn
		case zone.Get(string(StartTaskBtn)).InBounds(msg):
			spawnTask(tasks.ExampleTask)
			clicked = StartTaskBtn
		default:
			clicked = ButtonNone
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		theSpinner, cmd = theSpinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "c":
			resetRunningTask()
		}
	}
	return m, nil
}

func ButtonStyler(content string, kind Button, disabled bool) string {
	btnFrame := lipgloss.NewStyle().
		Width(24).
		Align(lipgloss.Center).
		Padding(0, 1).
		Border(lipgloss.NormalBorder())

	baseBtnStyle := btnFrame.
		Foreground(lipgloss.Color("#FFF7DB"))

	activeBtnStyle := btnFrame.
		Background(lipgloss.Color("#F25D94")).
		BorderBackground(lipgloss.Color("#F25D94")).
		Foreground(lipgloss.Color("#FFF7DB")).
		Bold(true)

	hoveredBtnStyle := btnFrame.
		Border(lipgloss.DoubleBorder()).
		Foreground(lipgloss.Color("#FFF7DB"))

	disabledBtnStyle := btnFrame.
		BorderForeground(lipgloss.Color("#525252")).
		Foreground(lipgloss.Color("#525252"))

	style := baseBtnStyle
	if disabled {
		style = disabledBtnStyle
	} else {
		if kind == hovered {
			style = hoveredBtnStyle
		}
		if kind == clicked {
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
					if isPolling {
						sb.WriteString(theSpinner.View())
						sb.WriteString("  ")
					} else {
						sb.WriteString("🛑 ")
					}
					sb.WriteString(lastViewPath)
					return sb.String()
				}(),
			),
		divider,
		lipgloss.NewStyle().Margin(0, 0, 0, 2).Render(
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				ButtonStyler(
					"Disable Polling",
					DisablePollingBtn,
					!isPolling,
				),
				ButtonStyler(
					"Enable Polling",
					EnablePollingBtn,
					isPolling,
				),
				ButtonStyler(
					"Cancel Task",
					CancelTaskBtn,
					!taskRunning,
				),
			),
		),
		divider,
		lipgloss.NewStyle().Margin(0, 0, 0, 2).Render(
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				ButtonStyler(
					"Artefact",
					ArtefactBtn,
					taskRunning,
				),
				ButtonStyler(
					"All 2 Lossless JPEG-XL",
					JxlBtn,
					taskRunning,
				),
				ButtonStyler(
					"PNG 2 Lossy JPEG-XL",
					LossyJxlBtn,
					taskRunning,
				),
			),
		),
		lipgloss.NewStyle().Margin(0, 0, 0, 2).Render(
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				ButtonStyler(
					"JPEG-XL 2 PNG, JPG",
					DjxlBtn,
					taskRunning,
				),
				ButtonStyler(
					"PAR2 from 7z",
					Par2Btn,
					taskRunning,
				),
				ButtonStyler(
					"Start Task",
					StartTaskBtn,
					taskRunning,
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

	p := tea.NewProgram(MainModel{}, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
