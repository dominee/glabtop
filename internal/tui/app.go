package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"glabtop/internal/cache"
	"glabtop/internal/config"
	"glabtop/internal/gitlab"
	"glabtop/internal/model"
	th "glabtop/internal/theme"
)

type secondTickMsg time.Time

type refreshPulseMsg struct{}

type fetchDoneMsg struct {
	gen  int
	snap *model.Snapshot
	err  error
}

type resolveDoneMsg struct {
	projects []model.ProjectRef
	err      error
}

// Model is the root Bubble Tea model.
type Model struct {
	cfg     config.File
	cfgPath string
	theme   *th.Theme
	styles  th.Styles
	git     *gitlab.Client
	store   *cache.Store

	w, h int

	snapshot   *model.Snapshot
	lastErr    error
	tr         model.TimeRange
	loading    bool
	offline    bool
	paused     bool
	projects   []model.ProjectRef
	resolveErr error

	showStats        bool
	showCommits      bool
	showIssues       bool
	detailPane       bool
	listCommitActive bool

	projectFilter string
	userFilter    string
	filterFocus   bool
	fi            textinput.Model

	commitList list.Model
	issueList  list.Model

	nextRefresh  time.Time
	refreshEvery time.Duration
	persist      config.PersistedUIState

	fetchGen int
}

func newDelegate(st th.Styles) list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.NormalTitle = st.Base
	d.Styles.NormalDesc = st.Inactive
	d.Styles.SelectedTitle = st.Selected
	d.Styles.SelectedDesc = st.Selected
	return d
}

// NewModel builds the initial TUI model.
func NewModel(cfg config.File, cfgPath string, thm *th.Theme, client *gitlab.Client, store *cache.Store, st config.PersistedUIState, offline bool, bootstrap *model.Snapshot) *Model {
	styles := thm.Styles()
	ti := textinput.New()
	ti.Placeholder = "substring (use path with / for project, else author)"
	ti.CharLimit = 200
	ti.Width = 60

	m := &Model{
		cfg:              cfg,
		cfgPath:          cfgPath,
		theme:            thm,
		styles:           styles,
		git:              client,
		store:            store,
		tr:               st.TimeRange,
		offline:          offline,
		showStats:        st.ShowStats,
		showCommits:      st.ShowCommits,
		showIssues:       st.ShowIssues,
		detailPane:       st.DetailPane,
		listCommitActive: true,
		fi:               ti,
		refreshEvery:     time.Duration(cfg.UI.RefreshIntervalSec) * time.Second,
		persist:          st,
		projectFilter:    cfg.Filters.ProjectSubstring,
		userFilter:       cfg.Filters.Username,
		commitList:       list.New([]list.Item{}, newDelegate(styles), 0, 0),
		issueList:        list.New([]list.Item{}, newDelegate(styles), 0, 0),
	}
	m.commitList.Title = "Commits"
	m.commitList.SetShowStatusBar(false)
	m.issueList.Title = "Closed issues"
	m.issueList.SetShowStatusBar(false)
	if bootstrap != nil {
		m.applySnapshot(bootstrap)
	}
	return m
}

// Init starts project resolution and timers.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		resolveProjectsCmd(m),
		tickEverySecond(),
		scheduleRefresh(m.refreshEvery),
	)
}

func cacheWarmCmd(store *cache.Store, tr model.TimeRange) tea.Cmd {
	if store == nil {
		return nil
	}
	return func() tea.Msg {
		want := model.WindowID(time.Now().UTC(), tr)
		snap, err := store.GetSnapshot(want)
		if err != nil || snap == nil {
			snap, _ = store.LastGoodSnapshot()
		}
		return fetchDoneMsg{gen: -1, snap: snap, err: nil}
	}
}

func tickEverySecond() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return secondTickMsg(t) })
}

func scheduleRefresh(d time.Duration) tea.Cmd {
	if d <= 0 {
		d = 10 * time.Minute
	}
	return tea.Tick(d, func(time.Time) tea.Msg { return refreshPulseMsg{} })
}

func resolveProjectsCmd(m *Model) tea.Cmd {
	if m.offline {
		return func() tea.Msg {
			var refs []model.ProjectRef
			for _, p := range m.cfg.Targets.Projects {
				refs = append(refs, model.ProjectRef{PathWithNamespace: p})
			}
			return resolveDoneMsg{projects: refs, err: nil}
		}
	}
	if m.git == nil {
		return func() tea.Msg { return resolveDoneMsg{nil, fmt.Errorf("no gitlab client")} }
	}
	groups := append([]string(nil), m.cfg.Targets.Groups...)
	projects := append([]string(nil), m.cfg.Targets.Projects...)
	client := m.git
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		p, err := client.ResolveProjects(ctx, groups, projects)
		return resolveDoneMsg{p, err}
	}
}

func (m *Model) fetchCmd() tea.Cmd {
	if m.offline || m.git == nil {
		return nil
	}
	if len(m.projects) == 0 {
		return nil
	}
	gen := m.fetchGen
	tr := m.tr
	pf, uf := m.projectFilter, m.userFilter
	projects := append([]model.ProjectRef(nil), m.projects...)
	client := m.git
	store := m.store

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		snap, err := client.FetchSnapshot(ctx, projects, tr, pf, uf)
		if err == nil && store != nil && snap != nil {
			_ = store.PutSnapshot(snap)
			since, e1 := time.Parse(time.RFC3339, snap.SinceRFC3339)
			until, e2 := time.Parse(time.RFC3339, snap.UntilRFC3339)
			if e1 == nil && e2 == nil && len(snap.Series) > 0 {
				_ = store.ReplaceTimeline(model.GranularityKey(tr), since, until, snap.Series)
			}
			for _, pr := range projects {
				_ = store.SetLastFetch(pr.PathWithNamespace, time.Now().Unix())
			}
		}
		return fetchDoneMsg{gen, snap, err}
	}
}

func (m *Model) applySnapshot(snap *model.Snapshot) {
	if snap == nil {
		return
	}
	m.snapshot = snap
	m.enrichSeriesFromDB()
	m.commitList.SetItems(commitItems(m.snapshot))
	m.issueList.SetItems(issueItems(m.snapshot))
}

func (m *Model) enrichSeriesFromDB() {
	if m.store == nil || m.snapshot == nil || len(m.snapshot.Series) > 0 {
		return
	}
	since, err1 := time.Parse(time.RFC3339, m.snapshot.SinceRFC3339)
	until, err2 := time.Parse(time.RFC3339, m.snapshot.UntilRFC3339)
	if err1 != nil || err2 != nil {
		return
	}
	tr := model.ParseTimeRange(m.snapshot.TimeRange)
	series, err := m.store.LoadSeries(tr, since, until)
	if err != nil || len(series) == 0 {
		return
	}
	m.snapshot.Series = series
	m.snapshot.Counts = model.NormalizeSeries(series)
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.filterFocus {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				m.filterFocus = false
				m.fi.Blur()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				m.filterFocus = false
				m.fi.Blur()
				line := strings.TrimSpace(m.fi.Value())
				if line == "" {
					m.projectFilter = ""
					m.userFilter = ""
				} else if strings.Contains(line, "/") {
					m.projectFilter = line
				} else {
					m.userFilter = line
				}
				m.fetchGen++
				m.loading = !m.offline && m.git != nil && len(m.projects) > 0
				return m, m.fetchCmd()
			}
		}
		var cmd tea.Cmd
		m.fi, cmd = m.fi.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.layoutLists()
		return m, nil

	case secondTickMsg:
		return m, nil

	case refreshPulseMsg:
		if m.paused || m.offline || m.loading {
			return m, scheduleRefresh(m.refreshEvery)
		}
		m.fetchGen++
		m.loading = true
		return m, tea.Batch(m.fetchCmd(), scheduleRefresh(m.refreshEvery))

	case resolveDoneMsg:
		m.resolveErr = msg.err
		m.projects = msg.projects
		if m.resolveErr == nil && len(m.projects) > 0 && !m.offline {
			m.fetchGen++
			m.loading = true
			return m, m.fetchCmd()
		}
		return m, nil

	case fetchDoneMsg:
		if msg.gen == -1 {
			if msg.snap != nil {
				want := model.WindowID(time.Now().UTC(), m.tr)
				if msg.snap.WindowID == want {
					m.applySnapshot(msg.snap)
				} else if m.snapshot == nil {
					m.applySnapshot(msg.snap)
				}
			}
			return m, nil
		}
		if msg.gen != m.fetchGen {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.lastErr = msg.err
		} else if msg.snap != nil {
			m.lastErr = nil
			m.applySnapshot(msg.snap)
			m.nextRefresh = time.Now().Add(m.refreshEvery)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.persist.TimeRange = m.tr
			m.persist.ShowStats = m.showStats
			m.persist.ShowCommits = m.showCommits
			m.persist.ShowIssues = m.showIssues
			m.persist.DetailPane = m.detailPane
			_ = config.SaveState(m.cfgPath, m.persist)
			return m, tea.Quit
		case "t":
			m.tr = m.tr.Next()
			m.fetchGen++
			cmds := []tea.Cmd{cacheWarmCmd(m.store, m.tr)}
			if !m.offline && m.git != nil && len(m.projects) > 0 {
				m.loading = true
				cmds = append(cmds, m.fetchCmd())
			}
			return m, tea.Batch(cmds...)
		case "r":
			m.fetchGen++
			if !m.offline && m.git != nil && len(m.projects) > 0 {
				m.loading = true
				m.nextRefresh = time.Now().Add(m.refreshEvery)
				return m, m.fetchCmd()
			}
			return m, nil
		case "p":
			m.paused = !m.paused
			return m, nil
		case "1":
			m.showStats = !m.showStats
			m.layoutLists()
			return m, nil
		case "2":
			m.showCommits = !m.showCommits
			m.layoutLists()
			return m, nil
		case "3":
			m.showIssues = !m.showIssues
			m.layoutLists()
			return m, nil
		case "d":
			m.detailPane = !m.detailPane
			m.layoutLists()
			return m, nil
		case "tab":
			m.listCommitActive = !m.listCommitActive
			return m, nil
		case "/":
			m.filterFocus = true
			m.fi.Focus()
			m.fi.SetValue("")
			return m, textinput.Blink
		}

		if routeToCommits(m, msg) {
			var cmd tea.Cmd
			m.commitList, cmd = m.commitList.Update(msg)
			return m, cmd
		}
		if routeToIssues(m, msg) {
			var cmd tea.Cmd
			m.issueList, cmd = m.issueList.Update(msg)
			return m, cmd
		}
	}

	var c0, c1 tea.Cmd
	m.commitList, c0 = m.commitList.Update(msg)
	m.issueList, c1 = m.issueList.Update(msg)
	if c0 != nil {
		return m, c0
	}
	return m, c1
}

func routeToCommits(m *Model, _ tea.KeyMsg) bool {
	if !m.showCommits {
		return false
	}
	if m.detailPane {
		return true
	}
	if m.showIssues {
		return m.listCommitActive
	}
	return true
}

func routeToIssues(m *Model, _ tea.KeyMsg) bool {
	if !m.showIssues {
		return false
	}
	if m.detailPane {
		return !m.showCommits || !m.listCommitActive
	}
	if m.showCommits {
		return !m.listCommitActive
	}
	return true
}

func (m *Model) layoutLists() {
	if m.w == 0 {
		return
	}
	headerRows := 2
	statsH := 0
	if m.showStats {
		statsH = 14
	}
	helpH := 2
	if m.filterFocus {
		helpH += 2
	}
	remain := m.h - headerRows - statsH - helpH
	if remain < 6 {
		remain = 6
	}

	const gapW = 1
	listH := remain

	var commitInnerW, issueInnerW int
	switch {
	case m.detailPane:
		full := m.w - 2
		if full < 1 {
			full = 1
		}
		if m.showCommits {
			commitInnerW = full
		}
		if m.showIssues {
			issueInnerW = full
		}
	case m.showCommits && m.showIssues:
		leftOut := (m.w - gapW) / 2
		rightOut := m.w - gapW - leftOut
		commitInnerW = leftOut - 2
		issueInnerW = rightOut - 2
	default:
		full := m.w - 2
		if full < 1 {
			full = 1
		}
		if m.showCommits {
			commitInnerW = full
		}
		if m.showIssues {
			issueInnerW = full
		}
	}
	if commitInnerW < 1 {
		commitInnerW = 1
	}
	if issueInnerW < 1 {
		issueInnerW = 1
	}

	if m.showCommits {
		m.commitList.SetSize(commitInnerW, listH)
	}
	if m.showIssues {
		m.issueList.SetSize(issueInnerW, listH)
	}
}

// View implements tea.Model.
func (m *Model) View() string {
	return renderView(m)
}
