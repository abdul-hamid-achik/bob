package studio

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/abdul-hamid-achik/bob/internal/engine"
)

var (
	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleActive   = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(lipgloss.Color("14"))
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleGood     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	styleWarn     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	styleBad      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	styleSelected = lipgloss.NewStyle().Bold(true).Reverse(true)
)

func (m Model) View() tea.View {
	content := m.render()
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	w, h := maxInt(1, m.width), maxInt(1, m.height)
	if m.quitting {
		return fitGrid("Bob studio closed.", w, h)
	}
	if w < 30 || h < 8 {
		return fitGrid(strings.Join([]string{
			styleTitle.Render("Bob studio"),
			"terminal too small",
			"resize to at least 30×8",
			"? help · r refresh · q quit",
		}, "\n"), w, h)
	}

	header := m.renderHeader(w)
	tabs := m.renderTabs(w)
	footer := m.renderFooter(w)
	bodyHeight := maxInt(1, h-3)
	var body string
	if m.help {
		body = m.renderHelp(w, bodyHeight)
	} else {
		switch m.active {
		case viewPlan:
			body = m.renderPlan(w, bodyHeight)
		case viewStats:
			body = m.renderStats(w, bodyHeight)
		default:
			body = m.renderOverview(w, bodyHeight)
		}
	}
	return fitGrid(strings.Join([]string{header, tabs, body, footer}, "\n"), w, h)
}

func (m Model) renderHeader(width int) string {
	workspace := safeSingleLine(m.workspace)
	state := "loading"
	recipe := ""
	if m.snapshot != nil {
		state = stateLabel(m.snapshot.Report.Repository.State)
		recipe = m.snapshot.Report.Repository.Recipe
		if recipe != "" && m.snapshot.Plan != nil {
			recipe = fmt.Sprintf("%s@%d", recipe, m.snapshot.Plan.Recipe.Version)
		}
	}
	parts := []string{styleTitle.Render("Bob studio"), clip(workspace, maxInt(12, width/2))}
	if recipe != "" {
		parts = append(parts, recipe)
	}
	parts = append(parts, renderState(state))
	if m.loading && m.snapshot != nil {
		parts = append(parts, styleDim.Render("refreshing"))
	}
	if m.refreshErr != "" {
		parts = append(parts, styleBad.Render("refresh failed"))
	}
	return truncateStyled(strings.Join(parts, " · "), width)
}

func (m Model) renderTabs(width int) string {
	tabs := []string{"1 Overview", "2 Plan", "3 Stats"}
	for i := range tabs {
		if viewID(i) == m.active {
			tabs[i] = styleActive.Render("[" + tabs[i] + "]")
		} else {
			tabs[i] = styleDim.Render(" " + tabs[i] + " ")
		}
	}
	return truncateStyled(strings.Join(tabs, "  "), width)
}

func (m Model) renderFooter(width int) string {
	text := "1/2/3 views · tab next · r refresh · ? help · q quit"
	if m.active == viewPlan {
		text = "↑/↓ select · PgUp/PgDn detail · a " + attentionLabel(m.attention) + " · r refresh · ? help · q quit"
	}
	if width < 72 {
		text = "↑/↓ · a filter · r refresh · ? help · q quit"
		if m.active != viewPlan {
			text = "tab view · ↑/↓ scroll · r refresh · ? help · q quit"
		}
	}
	if m.refreshErr != "" && m.snapshot != nil {
		captured := "unknown time"
		if !m.snapshot.CapturedAt.IsZero() {
			captured = m.snapshot.CapturedAt.Local().Format("15:04:05")
		}
		text = "refresh failed; showing snapshot from " + captured + " · r retry · q quit"
	}
	return styleDim.Render(clip(text, width))
}

func (m Model) renderOverview(width, height int) string {
	if m.snapshot == nil {
		return panel("Overview", m.loadingLines(), width, height)
	}
	if m.singlePane || width < 80 || height < 14 {
		return panel("Overview", windowLines(m.overviewLines(), m.overviewOff, maxInt(0, height-2)), width, height)
	}

	topHeight := maxInt(6, height/2)
	bottomHeight := height - topHeight
	leftWidth := clampInt(width/2, 34, width-34)
	rightWidth := width - leftWidth
	repository := panel("Repository", m.repositoryLines(), leftWidth, topHeight)
	integrations := panel("Integrations", m.integrationLines(), rightWidth, topHeight)
	top := lipgloss.JoinHorizontal(lipgloss.Top, repository, integrations)
	bottom := panel("Warnings & next actions", windowLines(m.noticeLines(), m.overviewOff, maxInt(0, bottomHeight-2)), width, bottomHeight)
	return fitBlock(lipgloss.JoinVertical(lipgloss.Left, top, bottom), width, height)
}

func (m Model) loadingLines() []string {
	if m.refreshErr != "" {
		return []string{
			"workspace unavailable",
			"",
			clip(safeSingleLine(m.refreshErr), maxInt(1, m.width-4)),
			"",
			"press r to retry or q to quit",
		}
	}
	return []string{"loading workspace…"}
}

func (m Model) overviewLines() []string {
	lines := append([]string{"Repository"}, m.repositoryLines()...)
	lines = append(lines, "", "Integrations")
	lines = append(lines, m.integrationLines()...)
	lines = append(lines, "", "Warnings & next actions")
	lines = append(lines, m.noticeLines()...)
	return lines
}

func (m Model) repositoryLines() []string {
	if m.snapshot == nil {
		return m.loadingLines()
	}
	repo := m.snapshot.Report.Repository
	lines := []string{
		"state       " + stateLabel(repo.State),
		fmt.Sprintf("ready       %t", repo.Ready),
		fmt.Sprintf("converged   %t", repo.Converged),
		fmt.Sprintf("lock change %t", repo.LockChanged),
		fmt.Sprintf("managed     %d", repo.ManagedFiles),
		fmt.Sprintf("conflicts   %d", repo.ConflictCount),
		fmt.Sprintf("actions     +%d  ~%d  =%d  ·%d  !%d", repo.Actions.Create, repo.Actions.Update, repo.Actions.Adopt, repo.Actions.Unchanged, repo.Actions.Conflict),
	}
	if repo.Error != "" {
		lines = append(lines, "", "error: "+safeSingleLine(repo.Error))
	}
	return lines
}

func (m Model) integrationLines() []string {
	if m.snapshot == nil {
		return []string{"integration state unavailable"}
	}
	if len(m.snapshot.Report.Integrations) == 0 {
		return []string{"no integrations reported"}
	}
	lines := make([]string, 0, len(m.snapshot.Report.Integrations)*2)
	for _, integration := range m.snapshot.Report.Integrations {
		binary := "unavailable"
		if integration.Available {
			binary = "available"
		}
		selected := "not selected"
		if integration.Selected {
			selected = "selected"
		}
		lines = append(lines, fmt.Sprintf("%s  %s · %s", integration.Name, selected, binary))
		lines = append(lines, fmt.Sprintf("  probe %s · index %s", stateLabel(integration.Probe.State), stateLabel(integration.Index.State)))
	}
	return lines
}

func (m Model) noticeLines() []string {
	if m.snapshot == nil {
		return m.loadingLines()
	}
	report := m.snapshot.Report
	lines := make([]string, 0, len(report.Warnings)+len(report.NextActions)*2+2)
	for _, warning := range report.Warnings {
		lines = append(lines, "warning: "+safeSingleLine(warning))
	}
	for _, action := range report.NextActions {
		authority := "read-only suggestion"
		if action.RequiresExplicitAuthority {
			authority = "explicit authority required"
		}
		lines = append(lines, "next: "+displayArgv(action.Argv))
		lines = append(lines, "  "+authority+" — "+safeSingleLine(action.Reason))
	}
	if len(lines) == 0 {
		return []string{"no warnings or next actions"}
	}
	return lines
}

func (m Model) renderPlan(width, height int) string {
	if m.snapshot == nil {
		return panel("Plan", m.loadingLines(), width, height)
	}
	if m.snapshot.Plan == nil {
		lines := []string{"plan unavailable", "", stateLabel(m.snapshot.Report.Repository.State)}
		if m.snapshot.Report.Repository.Error != "" {
			lines = append(lines, safeSingleLine(m.snapshot.Report.Repository.Error))
		}
		return panel("Plan", lines, width, height)
	}

	actions := m.visibleActions()
	if len(actions) == 0 {
		status := "clean — every managed file is unchanged"
		if m.snapshot.Plan.LockChanged {
			status = "lock update pending — managed files are unchanged"
		}
		lines := []string{status, fmt.Sprintf("%d managed files", len(m.snapshot.Plan.Actions)), "", "press a to show all actions"}
		return panel("Plan · "+attentionLabel(m.attention), lines, width, height)
	}

	if m.singlePane || width < 80 || height < 14 {
		contentHeight := maxInt(0, height-2)
		listRows := clampInt(contentHeight/3, 1, maxInt(1, contentHeight-3))
		lines := []string{"Actions"}
		lines = append(lines, m.actionListLines(listRows)...)
		lines = append(lines, "", "Selected action")
		detailHeight := maxInt(0, contentHeight-len(lines))
		lines = append(lines, windowLines(m.actionDetailLines(), m.detailOff, detailHeight)...)
		return panel("Plan · "+attentionLabel(m.attention), lines, width, height)
	}

	listWidth := clampInt(width*2/5, 34, 52)
	detailWidth := width - listWidth
	list := panel("Plan · "+attentionLabel(m.attention), m.actionListLines(maxInt(0, height-2)), listWidth, height)
	detail := panel("Action detail", windowLines(m.actionDetailLines(), m.detailOff, maxInt(0, height-2)), detailWidth, height)
	return fitBlock(lipgloss.JoinHorizontal(lipgloss.Top, list, detail), width, height)
}

func (m Model) actionListLines(available int) []string {
	actions := m.visibleActions()
	if available <= 0 || len(actions) == 0 {
		return nil
	}
	start := 0
	if m.cursor >= available {
		start = m.cursor - available + 1
	}
	lines := make([]string, 0, available)
	for i := start; i < len(actions) && len(lines) < available; i++ {
		action := actions[i]
		marker := actionMarker(action.Kind)
		row := fmt.Sprintf("%s %-9s %s", marker, action.Kind, safeSingleLine(action.Path))
		if i == m.cursor {
			row = styleSelected.Render(clip("▸ "+row, maxInt(1, m.width)))
		} else {
			row = "  " + row
		}
		lines = append(lines, row)
	}
	return lines
}

func (m Model) actionDetailLines() []string {
	action, ok := m.selectedAction()
	if !ok {
		return []string{"select an action"}
	}
	lines := []string{
		"path     " + safeSingleLine(action.Path),
		"kind     " + string(action.Kind),
		"reason   " + safeSingleLine(action.Reason),
		fmt.Sprintf("mode     %s → %s", modeString(action.CurrentMode), modeString(action.DesiredMode)),
	}
	if action.CurrentSHA256 != "" {
		lines = append(lines, "current  "+action.CurrentSHA256)
	}
	if action.LockedSHA256 != "" {
		lines = append(lines, "locked   "+action.LockedSHA256)
	}
	if action.DesiredSHA256 != "" {
		lines = append(lines, "desired  "+action.DesiredSHA256)
	}
	if action.DesiredPreview != "" {
		lines = append(lines, "", "desired preview (bounded)")
		lines = append(lines, safeMultiline(action.DesiredPreview)...)
	}
	return lines
}

func (m Model) renderStats(width, height int) string {
	if m.snapshot == nil {
		return panel("Stats", m.loadingLines(), width, height)
	}
	if m.singlePane || width < 80 || height < 12 {
		return panel("Stats", windowLines(m.statsLines(), m.statsOff, maxInt(0, height-2)), width, height)
	}
	leftWidth := clampInt(width/2, 30, width-30)
	rightWidth := width - leftWidth
	left := panel("Totals", m.statsTotalLines(), leftWidth, height)
	right := panel("Per operation", windowLines(m.statsOperationLines(), m.statsOff, maxInt(0, height-2)), rightWidth, height)
	return fitBlock(lipgloss.JoinHorizontal(lipgloss.Top, left, right), width, height)
}

func (m Model) statsLines() []string {
	lines := append([]string{"Totals"}, m.statsTotalLines()...)
	lines = append(lines, "", "Per operation")
	return append(lines, m.statsOperationLines()...)
}

func (m Model) statsTotalLines() []string {
	stats := m.snapshot.Stats
	status := "disabled"
	window := "No local usage events are being recorded."
	if stats.Enabled {
		status = "enabled · local only"
		window = fmt.Sprintf("Rolling %d-day aggregate.", stats.WindowDays)
	}
	return []string{
		"telemetry   " + status,
		fmt.Sprintf("operations  %d", stats.Total),
		fmt.Sprintf("success     %d", stats.Success),
		fmt.Sprintf("errors      %d", stats.Errors),
		fmt.Sprintf("conflicts   %d", stats.Conflicts),
		"",
		window,
		"Source-supplied aggregate only.",
		"Studio does not write telemetry.",
	}
}

func (m Model) statsOperationLines() []string {
	stats := m.snapshot.Stats
	keys := make([]string, 0, len(stats.PerOperation))
	for key := range stats.PerOperation {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return []string{"no operation history reported"}
	}
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%-14s %d", safeSingleLine(key), stats.PerOperation[key]))
	}
	return lines
}

func (m Model) renderHelp(width, height int) string {
	lines := []string{
		"Bob Studio is read-only.",
		"It never applies a plan or runs specialist probes.",
		"",
		"1 / 2 / 3       overview / plan / stats",
		"tab / shift+tab next / previous view",
		"↑/k  ↓/j       select or scroll",
		"g/home  G/end   first / last",
		"PgUp/PgDn      scroll detail",
		"a               attention / all plan actions",
		"r               refresh offline snapshot",
		"? / esc         close help",
		"q / ctrl+c      quit",
	}
	return panel("Help", windowLines(lines, 0, maxInt(0, height-2)), width, height)
}

func renderState(state string) string {
	upper := strings.ToUpper(state)
	switch state {
	case "clean", "fresh", "complete":
		return styleGood.Render(upper)
	case "conflicted", "invalid manifest", "plan error", "failed", "timed out", "invalid output", "wrong project":
		return styleBad.Render(upper)
	case "drifted", "stale", "unavailable", "not indexed":
		return styleWarn.Render(upper)
	default:
		return styleDim.Render(upper)
	}
}

func actionMarker(kind engine.ActionKind) string {
	switch kind {
	case engine.ActionCreate:
		return "+"
	case engine.ActionUpdate:
		return "~"
	case engine.ActionAdopt:
		return "="
	case engine.ActionConflict:
		return "!"
	default:
		return "·"
	}
}

func modeString(mode fmt.Stringer) string {
	if mode == nil {
		return "none"
	}
	value := mode.String()
	if value == "----------" {
		return "none"
	}
	return value
}

func panel(title string, lines []string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if width < 4 || height < 2 {
		return fitBlock(strings.Join(lines, "\n"), width, height)
	}
	innerWidth := width - 2
	titleWidth := maxInt(0, innerWidth-3)
	topText := "─"
	if titleWidth > 0 {
		title = clip(safeSingleLine(title), titleWidth)
		topText = "─ " + title + " "
	}
	top := "┌" + topText + strings.Repeat("─", maxInt(0, innerWidth-ansi.StringWidth(topText))) + "┐"
	bottom := "└" + strings.Repeat("─", innerWidth) + "┘"
	contentHeight := height - 2
	rows := make([]string, 0, height)
	rows = append(rows, top)
	for i := 0; i < contentHeight; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		line = truncateStyled(line, innerWidth)
		line += strings.Repeat(" ", maxInt(0, innerWidth-ansi.StringWidth(line)))
		rows = append(rows, "│"+line+"│")
	}
	rows = append(rows, bottom)
	return strings.Join(rows, "\n")
}

func windowLines(lines []string, offset, height int) []string {
	if height <= 0 || len(lines) == 0 {
		return nil
	}
	maximum := maxInt(0, len(lines)-height)
	offset = clampInt(offset, 0, maximum)
	end := minInt(len(lines), offset+height)
	return lines[offset:end]
}

func displayArgv(argv []string) string {
	parts := make([]string, 0, len(argv))
	for _, arg := range argv {
		arg = safeSingleLine(arg)
		if strings.ContainsAny(arg, " \t\n\"'") {
			parts = append(parts, strconv.Quote(arg))
		} else {
			parts = append(parts, arg)
		}
	}
	return strings.Join(parts, " ")
}

func safeSingleLine(value string) string {
	value = ansi.Strip(value)
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		default:
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}
	}, value)
}

func safeMultiline(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	for i := range lines {
		lines[i] = safeSingleLine(strings.ReplaceAll(lines[i], "\t", "    "))
	}
	return lines
}

func clip(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(safeSingleLine(value), width, "…")
}

func truncateStyled(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(value, width, "…")
}

func fitBlock(value string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimSuffix(value, "\n"), "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = truncateStyled(lines[i], width)
	}
	return strings.Join(lines, "\n")
}

func fitGrid(value string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimSuffix(value, "\n"), "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i := range lines {
		lines[i] = truncateStyled(lines[i], width)
		lines[i] += strings.Repeat(" ", maxInt(0, width-ansi.StringWidth(lines[i])))
	}
	return strings.Join(lines, "\n")
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
