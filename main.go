package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	stateList state = iota
	stateConfirm
)

type Model struct {
	state     state
	table     table.Model
	branches  []Branch
	repoPath  string
	keys      keyMap
	err       error
	showError bool
	selected  map[int]bool
	cursor    int
}

type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Delete key.Binding
	Quit   key.Binding
	Help   key.Binding
	Yes    key.Binding
	No     key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Select: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "select/deselect"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete selected"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Yes: key.NewBinding(
			key.WithKeys("y", "Y"),
			key.WithHelp("y", "yes"),
		),
		No: key.NewBinding(
			key.WithKeys("n", "N", "esc"),
			key.WithHelp("n/esc", "no"),
		),
	}
}

var (
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")).
			Bold(true)

	mergedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("34"))

	unmergedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208"))

	protectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Strikethrough(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Width(50)
)

func initialModel() Model {
	if err := validateGitEnvironment(); err != nil {
		return Model{
			err:       err,
			showError: true,
		}
	}

	branches, err := getAllBranches()
	if err != nil {
		return Model{
			err:       err,
			showError: true,
		}
	}

	repoPath, err := getRepositoryPath()
	if err != nil {
		repoPath = "Unknown"
	}

	sort.Slice(branches, func(i, j int) bool {
		return branches[i].LastCommitDate.After(branches[j].LastCommitDate)
	})

	columns := []table.Column{
		{Title: "Select", Width: 8},
		{Title: "Branch", Width: 25},
		{Title: "Status", Width: 10},
		{Title: "Date", Width: 12},
		{Title: "Commits", Width: 8},
		{Title: "Message", Width: 30},
	}

	rows := make([]table.Row, len(branches))
	for i, branch := range branches {
		checkbox := "[ ]"
		if branch.Selected {
			checkbox = "[x]"
		}
		if branch.IsMain {
			checkbox = "[-]"
		}

		status := "Unmerged"
		if branch.IsMerged {
			status = "Merged"
		}
		if branch.IsMain {
			status = "-"
		}

		commitsStr := fmt.Sprintf("%d↑", branch.CommitsAhead)
		if branch.IsMain {
			commitsStr = "-"
		}

		rows[i] = table.Row{
			checkbox,
			branch.Name,
			status,
			branch.LastCommitDate.Format("2006-01-02"),
			commitsStr,
			truncateString(branch.LastCommitMsg, 30),
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return Model{
		state:    stateList,
		table:    t,
		branches: branches,
		repoPath: repoPath,
		keys:     defaultKeyMap(),
		selected: make(map[int]bool),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case stateList:
			return m.updateList(msg)
		case stateConfirm:
			return m.updateConfirm(msg)
		}
	case tea.WindowSizeMsg:
		m.table.SetWidth(msg.Width - 4)
		m.table.SetHeight(msg.Height - 8)
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Select):
		cursor := m.table.Cursor()
		if cursor >= 0 && cursor < len(m.branches) && !m.branches[cursor].IsMain {
			m.branches[cursor].Selected = !m.branches[cursor].Selected
			m.selected[cursor] = m.branches[cursor].Selected
			m = m.updateTableRows()
		}
		return m, nil
	case key.Matches(msg, m.keys.Delete):
		selectedBranches := m.getSelectedBranches()
		if len(selectedBranches) > 0 {
			m.state = stateConfirm
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Yes):
		selectedBranches := m.getSelectedBranches()
		branchNames := make([]string, len(selectedBranches))
		for i, branch := range selectedBranches {
			branchNames[i] = branch.Name
		}

		err := deleteBranches(branchNames)
		if err != nil {
			m.err = err
			m.showError = true
			m.state = stateList
			return m, nil
		}

		branches, err := getAllBranches()
		if err != nil {
			m.err = err
			m.showError = true
			m.state = stateList
			return m, nil
		}

		m.branches = branches
		m.selected = make(map[int]bool)
		m = m.updateTableRows()
		m.state = stateList

	case key.Matches(msg, m.keys.No):
		m.state = stateList
	}

	return m, nil
}

func (m Model) View() string {
	if m.showError {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	switch m.state {
	case stateList:
		return m.viewList()
	case stateConfirm:
		return m.viewConfirm()
	}

	return ""
}

func (m Model) viewList() string {
	title := titleStyle.Render("Git Branch Cleaner")
	repo := fmt.Sprintf("Repository: %s", m.repoPath)

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("space: select • d: delete selected • q: quit • ↑/↓: navigate")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		repo,
		"",
		baseStyle.Render(m.table.View()),
		"",
		help,
	)

	return content
}

func (m Model) viewConfirm() string {
	selectedBranches := m.getSelectedBranches()

	var content strings.Builder
	content.WriteString("The following branches will be deleted:\n\n")

	hasUnmerged := false
	for _, branch := range selectedBranches {
		status := "Merged"
		if !branch.IsMerged {
			status = "Unmerged"
			hasUnmerged = true
		}
		content.WriteString(fmt.Sprintf("• %s (%s)\n", branch.Name, status))
	}

	if hasUnmerged {
		content.WriteString("\n⚠️  Warning: Some branches are unmerged!\n")
	}

	content.WriteString("\nContinue? (y/N)")

	dialog := dialogStyle.Render(content.String())

	return lipgloss.Place(
		80, 24,
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}

func (m Model) getSelectedBranches() []Branch {
	var selected []Branch
	for _, branch := range m.branches {
		if branch.Selected {
			selected = append(selected, branch)
		}
	}
	return selected
}

func (m Model) updateTableRows() Model {
	rows := make([]table.Row, len(m.branches))
	for i, branch := range m.branches {
		checkbox := "[ ]"
		if branch.Selected {
			checkbox = "[x]"
		}
		if branch.IsMain {
			checkbox = "[-]"
		}

		status := "Unmerged"
		if branch.IsMerged {
			status = "Merged"
		}
		if branch.IsMain {
			status = "-"
		}

		commitsStr := fmt.Sprintf("%d↑", branch.CommitsAhead)
		if branch.IsMain {
			commitsStr = "-"
		}

		rows[i] = table.Row{
			checkbox,
			branch.Name,
			status,
			branch.LastCommitDate.Format("2006-01-02"),
			commitsStr,
			truncateString(branch.LastCommitMsg, 30),
		}
	}
	m.table.SetRows(rows)
	return m
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
