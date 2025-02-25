package helmutil

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

type updateTUI struct {
	err             error
	startedCharts   []string
	completedCharts []string
	erroredCharts   []string
	spinner         spinner.Model
	progress        progress.Model
	totalCharts     int
	width           int
	height          int
	mu              sync.RWMutex
	done            bool
}

func newUpdateTUI() *updateTUI {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	s := spinner.New()
	s.Style = spinnerStyle

	return &updateTUI{
		startedCharts:   []string{},
		completedCharts: []string{},
		erroredCharts:   []string{},
		spinner:         s,
		progress:        p,
		mu:              sync.RWMutex{},
	}
}

func (m *updateTUI) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.progress.SetPercent(0))
}

//nolint:ireturn // Third-party.
func (m *updateTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		}

	case teaMsgWriteLog:
		return m, writeLog(msg, m.width)

	case teaMsgSetChartTotal:
		m.mu.Lock()
		defer m.mu.Unlock()

		m.totalCharts = int(msg)

	case teaMsgUpdatingChart:
		m.mu.Lock()
		defer m.mu.Unlock()

		chart := string(msg)
		m.startedCharts = append(m.startedCharts, chart)

	case teaMsgUpdatedChart:
		m.mu.Lock()
		defer m.mu.Unlock()

		icon := checkMark
		if msg.err != nil {
			m.erroredCharts = append(m.erroredCharts, msg.chart)
			icon = errorMark
		}

		m.completedCharts = append(m.completedCharts, msg.chart)
		completedCount := len(m.completedCharts)
		progressCmd := m.progress.SetPercent(float64(completedCount) / float64(m.totalCharts))

		if m.totalCharts == completedCount {
			m.done = true

			return m, tea.Sequence(
				tea.Printf("%s %s", icon, msg.chart),
				finalPause(),
				tea.Quit,
			)
		}

		return m, tea.Batch(
			progressCmd,
			tea.Printf("%s %s", icon, msg.chart),
		)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)

		return m, cmd

	case progress.FrameMsg:
		newModel, cmd := m.progress.Update(msg)
		if newModel, ok := newModel.(progress.Model); ok {
			m.progress = newModel
		}

		return m, cmd

	case error:
		m.mu.Lock()
		defer m.mu.Unlock()

		m.err = msg

		return m, tea.Sequence(finalPause(), tea.Quit)
	}

	return m, nil
}

func (m *updateTUI) View() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return getErrorMessage(m.err, m.width)
	}

	completedCount := len(m.completedCharts)

	if m.done {
		return doneStyle.Render(fmt.Sprintf("Done! Updated %d charts.\n", completedCount))
	}

	w := lipgloss.Width(strconv.Itoa(m.totalCharts))
	chartCount := fmt.Sprintf(" %*d/%*d", w, completedCount, w, m.totalCharts)

	prog := m.progress.View()
	progRendered := progressStyle.Render(prog + chartCount)
	progCellsRemaining := max(0, m.width-lipgloss.Width(progRendered))
	gap := strings.Repeat(" ", progCellsRemaining)
	progOut := progRendered + gap + "\n"

	inProgressCharts := differenceStringSlices(m.startedCharts, m.completedCharts)

	spinners := []string{}
	for _, chart := range inProgressCharts {
		spin := m.spinner.View() + " "
		cellsAvail := max(0, m.width-lipgloss.Width(spin))

		chartName := currentChartNameStyle.Render(chart)
		info := lipgloss.NewStyle().MaxWidth(cellsAvail).Render("Updating " + chartName)

		cellsRemaining := max(0, m.width-lipgloss.Width(spin+info))
		gap := strings.Repeat(" ", cellsRemaining)

		spinners = append(spinners, spin+info+gap)
	}

	return strings.Join(spinners, "\n") + "\n" + progOut
}

func differenceStringSlices(a, b []string) []string {
	difference := []string{}

	for _, x := range a {
		if !slices.Contains(b, x) {
			difference = append(difference, x)
		}
	}

	return difference
}
