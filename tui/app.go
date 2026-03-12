package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Mratuysl/kam/ai"
	"github.com/Mratuysl/kam/k8s"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	labelStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#73F59F"))
	commandStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Background(lipgloss.Color("#1a1a1a")).Padding(0, 1)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Bold(true)
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB700")).Bold(true)
	faintStyle   = lipgloss.NewStyle().Faint(true)
	inputBoxStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#7D56F4")).Padding(0, 1)
	outputBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#383838")).Padding(0, 1)
)

type aiResponseMsg struct {
	response *AIResponse
	err      error
}

type k8sResultMsg struct {
	results []*k8s.CommandResult
	err     error
}

type contextLoadedMsg string

type AIResponse struct {
	Commands    []string `json:"commands"`
	Explanation string   `json:"explanation"`
	Warning     string   `json:"warning"`
	Dangerous   bool     `json:"dangerous"`
}

type state int

const (
	stateInput state = iota
	stateThinking
	stateConfirm
	stateExecuting
	stateResult
)

type Model struct {
	input      textinput.Model
	viewport   viewport.Model
	spinner    spinner.Model
	aiProvider ai.Provider
	k8sClient  *k8s.Client
	state      state
	currentCtx string
	lastQuery  string
	lastResp   *AIResponse
	width      int
	height     int
}

func New(provider ai.Provider, client *k8s.Client) *Model {
	ti := textinput.New()
	ti.Placeholder = "Ne yapmak istiyorsun? (örn: prod'daki podları göster)"
	ti.Focus()
	ti.Width = 80

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	vp := viewport.New(80, 20)

	return &Model{
		input:      ti,
		viewport:   vp,
		spinner:    sp,
		aiProvider: provider,
		k8sClient:  client,
		state:      stateInput,
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.loadContext())
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 10
		m.input.Width = msg.Width - 6

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case contextLoadedMsg:
		m.currentCtx = string(msg)

	case aiResponseMsg:
		if msg.err != nil {
			m.state = stateResult
			m.viewport.SetContent(errorStyle.Render("✗ AI Hatası: " + msg.err.Error()))
			return m, nil
		}
		m.lastResp = msg.response
		if msg.response.Dangerous {
			m.state = stateConfirm
			m.renderConfirm(msg.response)
		} else {
			m.state = stateExecuting
			return m, tea.Batch(m.spinner.Tick, m.executeCommands(msg.response.Commands))
		}

	case k8sResultMsg:
		m.state = stateResult
		if msg.err != nil {
			m.viewport.SetContent(errorStyle.Render("✗ kubectl Hatası: " + msg.err.Error()))
		} else {
			m.renderResults(m.lastResp, msg.results)
		}
	}

	var cmds []tea.Cmd
	if m.state == stateInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		if m.state != stateInput {
			m.state = stateInput
			m.input.SetValue("")
			m.input.Focus()
			m.viewport.SetContent("")
			return m, textinput.Blink
		}
	case tea.KeyEnter:
		switch m.state {
		case stateInput:
			return m.submitQuery()
		case stateConfirm:
			m.state = stateExecuting
			return m, tea.Batch(m.spinner.Tick, m.executeCommands(m.lastResp.Commands))
		case stateResult:
			m.state = stateInput
			m.input.SetValue("")
			m.input.Focus()
			m.viewport.SetContent("")
			return m, textinput.Blink
		}
	}
	// stateInput'ta karakter tuşlarını textinput'a ilet
	if m.state == stateInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) submitQuery() (tea.Model, tea.Cmd) {
	query := strings.TrimSpace(m.input.Value())
	if query == "" {
		return m, nil
	}
	m.lastQuery = query
	m.state = stateThinking
	return m, tea.Batch(m.spinner.Tick, m.askAI(query))
}

func (m *Model) loadContext() tea.Cmd {
	return func() tea.Msg {
		ctx, err := m.k8sClient.GetCurrentContext(context.Background())
		if err != nil {
			return contextLoadedMsg("unknown")
		}
		return contextLoadedMsg(ctx)
	}
}

func (m *Model) askAI(query string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		prompt := fmt.Sprintf("Kubectl context: %s\n\nİstek: %s", m.currentCtx, query)
		raw, err := m.aiProvider.Complete(ctx, ai.K8sSystemPrompt, prompt)
		if err != nil {
			return aiResponseMsg{err: err}
		}
		raw = strings.TrimSpace(raw)
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)

		var response AIResponse
		if err := json.Unmarshal([]byte(raw), &response); err != nil {
			var generic map[string]interface{}
			if err2 := json.Unmarshal([]byte(raw), &generic); err2 == nil {
				if cmd, ok := generic["command"].(string); ok {
					response.Commands = []string{cmd}
				}
				if exp, ok := generic["explanation"].(string); ok {
					response.Explanation = exp
				} else if desc, ok := generic["description"].(string); ok {
					response.Explanation = desc
				}
				response.Warning = "Güvenli"
				response.Dangerous = false
			} else {
				return aiResponseMsg{err: fmt.Errorf("yanıt parse edilemedi: %s", raw)}
			}
		}
		return aiResponseMsg{response: &response}
	}
}

func (m *Model) executeCommands(commands []string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		results, err := m.k8sClient.RunMultiple(ctx, commands)
		return k8sResultMsg{results: results, err: err}
	}
}

func (m *Model) renderResults(resp *AIResponse, results []*k8s.CommandResult) {
	var sb strings.Builder
	sb.WriteString(labelStyle.Render("📋 "+resp.Explanation) + "\n\n")
	for _, result := range results {
		sb.WriteString(commandStyle.Render("⚡ "+result.Command) + "\n\n")
		if result.ExitCode != 0 {
			sb.WriteString(errorStyle.Render("✗ Hata") + "\n" + result.Stderr + "\n")
		} else {
			output := strings.TrimSpace(result.Stdout)
			if output == "" {
				output = "(boş çıktı)"
			}
			sb.WriteString(outputBoxStyle.Render(output) + "\n")
		}
		sb.WriteString(faintStyle.Render(fmt.Sprintf("⏱ %s", result.Duration.Round(time.Millisecond))) + "\n\n")
	}
	sb.WriteString(faintStyle.Render("Enter: yeni sorgu  •  Esc: temizle  •  Ctrl+C: çıkış"))
	m.viewport.SetContent(sb.String())
	m.viewport.GotoTop()
}

func (m *Model) renderConfirm(resp *AIResponse) {
	var sb strings.Builder
	sb.WriteString(warningStyle.Render("⚠️  TEHLİKELİ OPERASYON") + "\n\n")
	sb.WriteString(resp.Warning + "\n\n")
	sb.WriteString(labelStyle.Render("Çalıştırılacak komutlar:") + "\n")
	for _, cmd := range resp.Commands {
		sb.WriteString(commandStyle.Render(cmd) + "\n")
	}
	sb.WriteString("\n" + warningStyle.Render("Enter = onayla  •  Esc = iptal"))
	m.viewport.SetContent(sb.String())
}

func (m *Model) View() string {
	if m.width == 0 {
		return "Yükleniyor..."
	}
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("⎈ KAM") +
		faintStyle.Render("  ctx: "+m.currentCtx+"  ") +
		faintStyle.Render(m.aiProvider.Name()) + "\n")
	sb.WriteString(strings.Repeat("─", m.width) + "\n")

	switch m.state {
	case stateInput:
		sb.WriteString(inputBoxStyle.Render(m.input.View()) + "\n")
		sb.WriteString(faintStyle.Render("Enter: gönder  •  Esc: temizle  •  Ctrl+C: çıkış") + "\n\n")
		sb.WriteString(m.viewport.View())
	case stateThinking:
		sb.WriteString(inputBoxStyle.Render(m.input.View()) + "\n\n")
		sb.WriteString(m.spinner.View() + "  AI düşünüyor...")
	case stateConfirm:
		sb.WriteString(m.viewport.View())
	case stateExecuting:
		sb.WriteString(m.spinner.View() + "  Komutlar çalıştırılıyor...")
	case stateResult:
		sb.WriteString(m.viewport.View())
	}
	return sb.String()
}
