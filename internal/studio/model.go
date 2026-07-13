package studio

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/bob/internal/engine"
)

type viewID int

const (
	viewOverview viewID = iota
	viewPlan
	viewStats
	viewCount
)

type snapshotLoadedMsg struct {
	request  uint64
	snapshot Snapshot
	err      error
}

// Model is Bob Studio's read-only Elm model. All filesystem work is delegated
// to Source through tea.Cmd; View only renders immutable snapshot data.
type Model struct {
	ctx         context.Context
	source      Source
	workspace   string
	singlePane  bool
	width       int
	height      int
	active      viewID
	snapshot    *Snapshot
	cursor      int
	detailOff   int
	overviewOff int
	statsOff    int
	attention   bool
	help        bool
	quitting    bool
	loading     bool
	queued      bool
	request     uint64
	refreshErr  string
}

// NewModel constructs a Studio model. The initial load begins in Init.
func NewModel(workspace string, source Source, singlePane bool) Model {
	return NewModelWithContext(context.Background(), workspace, source, singlePane)
}

// NewModelWithContext constructs a model whose asynchronous reads are canceled
// with the owning program.
func NewModelWithContext(ctx context.Context, workspace string, source Source, singlePane bool) Model {
	if ctx == nil {
		ctx = context.Background()
	}
	if source == nil {
		source = NewRepositorySource(nil)
	}
	return Model{
		ctx:        ctx,
		source:     source,
		workspace:  workspace,
		singlePane: singlePane,
		width:      100,
		height:     30,
		attention:  true,
		loading:    true,
		request:    1,
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadCmd(m.request)
}

func (m Model) loadCmd(request uint64) tea.Cmd {
	return func() tea.Msg {
		snapshot, err := m.source.Load(m.ctx, m.workspace)
		return snapshotLoadedMsg{request: request, snapshot: snapshot, err: err}
	}
}

func (m *Model) requestRefresh(queueIfBusy bool) tea.Cmd {
	if m.loading {
		if queueIfBusy {
			m.queued = true
		}
		return nil
	}
	m.request++
	m.loading = true
	return m.loadCmd(m.request)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = maxInt(1, msg.Width)
		m.height = maxInt(1, msg.Height)
		m.clampOffsets()
		return m, nil
	case snapshotLoadedMsg:
		return m.handleSnapshot(msg)
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleSnapshot(msg snapshotLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.request != m.request {
		return m, nil
	}
	m.loading = false
	selectedPath := m.selectedPath()
	if msg.err != nil {
		m.refreshErr = msg.err.Error()
	} else {
		snapshot := msg.snapshot
		if snapshot.Stats.PerOperation == nil {
			snapshot.Stats.PerOperation = map[string]int{}
		}
		m.snapshot = &snapshot
		m.refreshErr = ""
		m.restoreSelection(selectedPath)
		m.clampOffsets()
	}
	var cmd tea.Cmd
	if m.queued {
		m.queued = false
		cmd = m.requestRefresh(false)
	}
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Keystroke()
	if m.help {
		switch key {
		case "?", "esc":
			m.help = false
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	switch key {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.help = true
		return m, nil
	case "esc":
		m.active = viewOverview
		return m, nil
	case "1":
		m.active = viewOverview
		return m, nil
	case "2":
		m.active = viewPlan
		return m, nil
	case "3":
		m.active = viewStats
		return m, nil
	case "tab":
		m.active = (m.active + 1) % viewCount
		return m, nil
	case "shift+tab":
		m.active = (m.active + viewCount - 1) % viewCount
		return m, nil
	case "r":
		return m, m.requestRefresh(true)
	case "a":
		if m.active == viewPlan {
			selected := m.selectedPath()
			m.attention = !m.attention
			m.restoreSelection(selected)
			m.detailOff = 0
		}
		return m, nil
	case "up", "k":
		if m.active == viewPlan {
			m.moveCursor(-1)
		} else {
			m.scrollActive(-1)
		}
		return m, nil
	case "down", "j":
		if m.active == viewPlan {
			m.moveCursor(1)
		} else {
			m.scrollActive(1)
		}
		return m, nil
	case "home", "g":
		if m.active == viewPlan {
			m.cursor = 0
			m.detailOff = 0
		} else {
			m.setActiveOffset(0)
		}
		return m, nil
	case "end", "G":
		if m.active == viewPlan {
			m.cursor = maxInt(0, len(m.visibleActions())-1)
			m.detailOff = 0
		} else {
			m.setActiveOffset(m.activeLineCount())
		}
		return m, nil
	case "pgup", "ctrl+u":
		m.scrollPage(-1)
		return m, nil
	case "pgdown", "ctrl+d":
		m.scrollPage(1)
		return m, nil
	}
	return m, nil
}

func (m *Model) moveCursor(delta int) {
	actions := m.visibleActions()
	if len(actions) == 0 {
		m.cursor = 0
		m.detailOff = 0
		return
	}
	m.cursor = clampInt(m.cursor+delta, 0, len(actions)-1)
	m.detailOff = 0
}

func (m *Model) scrollActive(delta int) {
	switch m.active {
	case viewOverview:
		m.overviewOff = clampInt(m.overviewOff+delta, 0, maxInt(0, len(m.overviewLines())-1))
	case viewStats:
		m.statsOff = clampInt(m.statsOff+delta, 0, maxInt(0, len(m.statsLines())-1))
	}
}

func (m *Model) scrollPage(direction int) {
	step := maxInt(1, m.height/2)
	switch m.active {
	case viewOverview:
		m.overviewOff = clampInt(m.overviewOff+direction*step, 0, maxInt(0, len(m.overviewLines())-1))
	case viewPlan:
		m.detailOff = clampInt(m.detailOff+direction*step, 0, maxInt(0, len(m.actionDetailLines())-1))
	case viewStats:
		m.statsOff = clampInt(m.statsOff+direction*step, 0, maxInt(0, len(m.statsLines())-1))
	}
}

func (m *Model) setActiveOffset(value int) {
	switch m.active {
	case viewOverview:
		m.overviewOff = value
	case viewStats:
		m.statsOff = value
	}
}

func (m Model) activeLineCount() int {
	switch m.active {
	case viewOverview:
		return len(m.overviewLines())
	case viewStats:
		return len(m.statsLines())
	default:
		return 0
	}
}

func (m Model) visibleActions() []engine.Action {
	if m.snapshot == nil || m.snapshot.Plan == nil {
		return nil
	}
	if !m.attention {
		return m.snapshot.Plan.Actions
	}
	result := make([]engine.Action, 0, len(m.snapshot.Plan.Actions))
	for _, action := range m.snapshot.Plan.Actions {
		if action.Kind != engine.ActionUnchanged {
			result = append(result, action)
		}
	}
	return result
}

func (m Model) selectedAction() (engine.Action, bool) {
	actions := m.visibleActions()
	if m.cursor < 0 || m.cursor >= len(actions) {
		return engine.Action{}, false
	}
	return actions[m.cursor], true
}

func (m Model) selectedPath() string {
	action, ok := m.selectedAction()
	if !ok {
		return ""
	}
	return action.Path
}

func (m *Model) restoreSelection(path string) {
	actions := m.visibleActions()
	if len(actions) == 0 {
		m.cursor = 0
		m.detailOff = 0
		return
	}
	for i, action := range actions {
		if action.Path == path {
			m.cursor = i
			return
		}
	}
	m.cursor = clampInt(m.cursor, 0, len(actions)-1)
}

func (m *Model) clampOffsets() {
	m.cursor = clampInt(m.cursor, 0, maxInt(0, len(m.visibleActions())-1))
	m.detailOff = clampInt(m.detailOff, 0, maxInt(0, len(m.actionDetailLines())-1))
	m.overviewOff = clampInt(m.overviewOff, 0, maxInt(0, len(m.overviewLines())-1))
	m.statsOff = clampInt(m.statsOff, 0, maxInt(0, len(m.statsLines())-1))
}

func attentionLabel(attention bool) string {
	if attention {
		return "attention"
	}
	return "all"
}

func stateLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return strings.ReplaceAll(value, "_", " ")
}
