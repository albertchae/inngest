package cli

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/inngest/inngestctl/pkg/execution/driver/dockerdriver"
)

var (
	tickDelay    = 10 * time.Millisecond
	warningDelay = 8 * time.Second
)

type BuilderUIOpts struct {
	BuildOpts      dockerdriver.BuildOpts
	QuitOnComplete bool
}

// NewBuilder renders UI for building an image.
func NewBuilder(ctx context.Context, opts BuilderUIOpts) (*BuilderUI, error) {
	p := progress.New(progress.WithDefaultGradient())
	b, err := dockerdriver.NewBuilder(ctx, opts.BuildOpts)
	return &BuilderUI{
		opts:     opts,
		Builder:  b,
		progress: p,
	}, err
}

type BuilderUI struct {
	opts BuilderUIOpts

	Builder  *dockerdriver.Builder
	buildErr error

	// warning is shown if the build takes a long time, or it takes a while
	// to progress from 0
	warning string
	start   time.Time

	progress progress.Model
}

func (b *BuilderUI) Init() tea.Cmd {

	// Start the build.
	b.buildErr = b.Builder.Start()
	b.start = time.Now()
	return tea.Tick(tickDelay, b.tick)
}

type progressMsg float64

func (b *BuilderUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlBackslash:
			return b, tea.Quit
		}
	case progressMsg:
		cmds = append(cmds, tea.Tick(tickDelay, b.tick))
	}

	m, cmd := b.progress.Update(msg)
	b.progress = m.(progress.Model)
	cmds = append(cmds, cmd)

	if b.Builder.Done() && b.opts.QuitOnComplete {
		cmds = append(cmds, tea.Quit)
	}

	return b, tea.Batch(cmds...)
}

func (b *BuilderUI) tick(t time.Time) tea.Msg {
	taken := time.Since(b.start)

	if taken > warningDelay && b.Builder.Progress() == 0 {
		b.warning = "This is taking some time.  Do you have internet?"
	}

	if taken > warningDelay*2 && b.Builder.Progress() == 0 {
		b.warning = "Like, a really long time :("
	}

	if taken > warningDelay*4 && b.Builder.Progress() == 0 {
		b.warning = "We need internet to pull image metadata.  Sorry, but it's not working now."
	}

	return progressMsg(b.Builder.Progress())

}

func (b *BuilderUI) View() string {
	if b.buildErr != nil {
		return RenderError(b.buildErr.Error())
	}

	s := &strings.Builder{}

	output := b.Builder.Output(1)
	if err := b.Builder.Error(); err != nil {
		output = "\n" + RenderError(err.Error())
	} else {
		output = TextStyle.Copy().Foreground(Feint).Render(output)
	}

	header := lipgloss.Place(
		50, 3,
		lipgloss.Left, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Top,
			b.progress.ViewAs(b.Builder.Progress()/100),
			TextStyle.Copy().Foreground(Feint).Render(b.Builder.ProgressText()),
			output,
		),
	)

	s.WriteString(header)

	if b.warning != "" {
		s.WriteString("\n")
		s.WriteString(TextStyle.Copy().Foreground(Orange).Render(b.warning))
	}

	return lipgloss.NewStyle().Padding(1, 0).Render(s.String())
}