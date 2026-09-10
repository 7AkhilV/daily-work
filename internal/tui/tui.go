package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/7AkhilV/daily-work/internal/clipboard"
	"github.com/7AkhilV/daily-work/internal/summary"
)

type mode int

const (
	modeMenu mode = iota
	modeEditPick
	modeEditText
	modeAdd
	modeRemove
)

var menuLabels = []string{
	"Edit",
	"Add task",
	"Remove task",
	"Regenerate",
	"Copy to clipboard",
	"Exit",
}

type Model struct {
	date         time.Time
	items        []summary.WorkItem
	mode         mode
	cursor       int
	pickCursor   int
	input        textinput.Model
	editIdx      int
	status       string
	quitting     bool
	onRegenerate func() ([]summary.WorkItem, error)
}

type regenerateMsg struct {
	items []summary.WorkItem
	err   error
}

func New(date time.Time, items []summary.WorkItem, onRegenerate func() ([]summary.WorkItem, error)) Model {
	ti := textinput.New()
	ti.Placeholder = "Project: description"
	ti.CharLimit = 300
	ti.Width = 60

	return Model{
		date:         date,
		items:        items,
		mode:         modeMenu,
		input:        ti,
		onRegenerate: onRegenerate,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case regenerateMsg:
		if msg.err != nil {
			m.status = "Regenerate failed: " + msg.err.Error()
			m.mode = modeMenu
			return m, nil
		}
		m.items = summary.MergePreservingManual(m.items, msg.items)
		m.status = "✓ Summary regenerated"
		m.mode = modeMenu
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		switch m.mode {
		case modeMenu:
			return m.updateMenu(msg)
		case modeEditPick, modeRemove:
			return m.updatePicker(msg)
		case modeEditText, modeAdd:
			return m.updateInput(msg)
		}
	}
	return m, nil
}

func (m Model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(menuLabels)-1 {
			m.cursor++
		}
	case "enter":
		switch menuLabels[m.cursor] {
		case "Edit":
			if len(m.items) == 0 {
				m.status = "No items to edit"
				return m, nil
			}
			m.pickCursor = 0
			m.mode = modeEditPick
			m.status = ""
		case "Add task":
			m.input.SetValue("")
			m.input.Focus()
			m.mode = modeAdd
			m.status = ""
			return m, textinput.Blink
		case "Remove task":
			if len(m.items) == 0 {
				m.status = "No items to remove"
				return m, nil
			}
			m.pickCursor = 0
			m.mode = modeRemove
			m.status = ""
		case "Regenerate":
			m.status = "Regenerating..."
			return m, m.cmdRegenerate()
		case "Copy to clipboard":
			text := summary.FormatSummary(m.date, m.items)
			if err := clipboard.Copy(text); err != nil {
				m.status = "Copy failed: " + err.Error()
			} else {
				m.status = "✓ Copied successfully"
			}
		case "Exit":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeMenu
		return m, nil
	case "up", "k":
		if m.pickCursor > 0 {
			m.pickCursor--
		}
	case "down", "j":
		if m.pickCursor < len(m.items)-1 {
			m.pickCursor++
		}
	case "enter":
		idx := m.pickCursor
		if idx < 0 || idx >= len(m.items) {
			m.mode = modeMenu
			return m, nil
		}
		if m.mode == modeRemove {
			m.items = append(m.items[:idx], m.items[idx+1:]...)
			m.status = "✓ Item removed"
			m.mode = modeMenu
			return m, nil
		}
		m.editIdx = idx
		m.input.SetValue(strings.TrimPrefix(m.items[idx].DisplayLine(), "- "))
		m.input.Focus()
		m.mode = modeEditText
		return m, textinput.Blink
	}
	return m, nil
}

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeMenu
		return m, nil
	case "enter":
		val := summary.ParseTaskInput(m.input.Value())
		if val == "" {
			m.status = "Empty task ignored"
			m.mode = modeMenu
			return m, nil
		}
		if m.mode == modeAdd {
			m.items = append(m.items, summary.WorkItem{Text: val, Manual: true})
			m.status = "✓ Task added"
		} else {
			item := m.items[m.editIdx]
			item.Text = val
			item.Edited = true
			m.items[m.editIdx] = item
			m.status = "✓ Item updated"
		}
		m.mode = modeMenu
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) cmdRegenerate() tea.Cmd {
	return func() tea.Msg {
		if m.onRegenerate == nil {
			return regenerateMsg{err: fmt.Errorf("regenerate not available")}
		}
		items, err := m.onRegenerate()
		return regenerateMsg{items: items, err: err}
	}
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	titleStyle := lipgloss.NewStyle().Bold(true)
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	sep := strings.Repeat("─", 40)

	b.WriteString(titleStyle.Render("Daily Work — " + summary.FormatDate(m.date)))
	b.WriteString("\n")
	b.WriteString(sep)
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		b.WriteString("(no work items)\n")
	} else {
		for _, it := range m.items {
			b.WriteString(it.DisplayLine())
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(sep)
	b.WriteString("\n\n")

	switch m.mode {
	case modeMenu:
		b.WriteString("What would you like to do?\n\n")
		for i, label := range menuLabels {
			if i == m.cursor {
				b.WriteString(cursorStyle.Render("❯ " + label))
			} else {
				b.WriteString("  " + label)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n(↑/↓ enter, ctrl+c quit)\n")
	case modeEditPick:
		b.WriteString("Edit which item?\n\n")
		b.WriteString(m.renderPicker())
		b.WriteString("\n(esc to cancel)\n")
	case modeRemove:
		b.WriteString("Remove which item?\n\n")
		b.WriteString(m.renderPicker())
		b.WriteString("\n(esc to cancel)\n")
	case modeEditText:
		b.WriteString("Edit task:\n\n")
		b.WriteString(m.input.View())
		b.WriteString("\n\n(enter to save, esc to cancel)\n")
	case modeAdd:
		b.WriteString("Add task:\n\n> ")
		b.WriteString(m.input.View())
		b.WriteString("\n\n(enter to save, esc to cancel)\n")
	}

	if m.status != "" {
		b.WriteString("\n")
		b.WriteString(m.status)
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) renderPicker() string {
	var b strings.Builder
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	for i, it := range m.items {
		line := it.DisplayLine()
		if i == m.pickCursor {
			b.WriteString(cursorStyle.Render("❯ " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) Items() []summary.WorkItem {
	return m.items
}

// Run starts the interactive session.
func Run(date time.Time, items []summary.WorkItem, onRegenerate func() ([]summary.WorkItem, error)) error {
	m := New(date, items, onRegenerate)
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}
