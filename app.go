package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/yourusername/kam/ai"
	"github.com/yourusername/kam/k8s"
)

// ─── Stiller ──────────────────────────────────────────────────────────────────

var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	danger    = lipgloss.AdaptiveColor{Light: "#FF0000", Dark: "#FF4444"}

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#73F59F"))

	outputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4444")).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB700")).
			Bold(true)

	commandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Background(lipgloss.Color("#1a1a1a")).
			Padding(0, 1)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(highlight).
			Padding(0, 1)
)

// ─── Mesajlar (Bubble Tea msg tipleri) ────────────────────────────────────────

type aiResponseMsg struct {
	response *AIResponse
	err      error
}

type k8sResultMsg struct {
	results []*k8s.CommandResult
	err     error
}

type confirmMsg struct {
	commands []string
}

// ─── AI Yanıt Yapısı ──────────────────────────────────────────────────────────

type AIResponse struct {
	Commands    []string `json:"commands"`
	Explanation string   `json:"explanation"`
	Warning     string   `json:"warning"`
	Dangerous   bool     `json:"dangerous"`
}

// ─── Uygulama Durumu ──────────────────────────────────────────────────────────

type state int

const (
	stateInput      state = iota // kullanıcı yazıyor
	stateThinking                // AI düşünüyor
	stateConfirm                 // tehlikeli komut onayı bekleniyor
	stateExecuting               // kubectl çalışıyor
	stateResult                  // sonuç gösteriliyor
)

type Model struct {
	// Bileşenler
	input    textarea.Model
	viewport viewport.Model
	spinner  spinner.Model

	// Servisler
	aiProvider ai.Provider
	k8sClient  *k8s.Client

	// Durum
	state       state
	currentCtx  string
	history     []HistoryEntry
	lastResponse *AIResponse
	width        int
	height       int
	err          error
}

type HistoryEntry struct {
	Query    string
	Response *AIResponse
	Results  []*k8s.CommandResult
	Time     time.Time
}

func New(provider ai.Provider, client *k8s.Client) *Model {
	// Textarea (kullanıcı girişi)
	ta := textarea.New()
	ta.Placeholder = "Ne yapmak istiyorsun? (örn: production'daki yüksek memory kullanan podları göster)"
	ta.Focus()
	ta.CharLimit = 500
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	// Spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	// Viewport (sonuçlar)
	vp := viewport.New(80, 20)

	return &Model{
		input:      ta,
		viewport:   vp,
		spinner:    sp,
		aiProvider: provider,
		k8sClient:  client,
		state:      stateInput,
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.loadContext(),
	)
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 12
		m.input.SetWidth(msg.Width - 4)

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case aiResponseMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateResult
			m.renderError(msg.err)
			return m, nil
		}
		m.lastResponse = msg.response
		if msg.response.Dangerous {
			m.state = stateConfirm
			m.renderConfirm(msg.response)
		} else {
			m.state = stateExecuting
			return m, m.executeCommands(msg.response.Commands)
		}

	case k8sResultMsg:
		m.state = stateResult
		if msg.err != nil {
			m.renderError(msg.err)
		} else {
			m.renderResults(m.lastResponse, msg.results)
		}

	case string: // context yüklendi
		m.currentCtx = msg
	}

	// Input ve viewport her zaman güncellenir
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
	case tea.KeyCtrlC, tea.KeyEsc:
		if m.state == stateInput {
			return m, tea.Quit
		}
		m.state = stateInput
		m.input.Focus()
		return m, textarea.Blink

	case tea.KeyCtrlD:
		return m, tea.Quit

	case tea.KeyEnter:
		if msg.Alt { // Alt+Enter = gönder
			return m.submitQuery()
		}
		if m.state == stateConfirm {
			// Onay verildi, çalıştır
			m.state = stateExecuting
			return m, m.executeCommands(m.lastResponse.Commands)
		}

	case tea.KeyCtrlN: // Ctrl+N = yeni sorgu
		m.state = stateInput
		m.input.Reset()
		m.input.Focus()
		return m, textarea.Blink
	}

	return m, nil
}

func (m *Model) submitQuery() (tea.Model, tea.Cmd) {
	query := strings.TrimSpace(m.input.Value())
	if query == "" {
		return m, nil
	}

	m.state = stateThinking
	m.err = nil

	return m, tea.Batch(
		m.spinner.Tick,
		m.askAI(query),
	)
}

// ─── Komutlar ─────────────────────────────────────────────────────────────────

func (m *Model) loadContext() tea.Cmd {
	return func() tea.Msg {
		ctx, err := m.k8sClient.GetCurrentContext(context.Background())
		if err != nil {
			return string("unknown")
		}
		return ctx
	}
}

func (m *Model) askAI(query string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		
		// Context bilgisini de prompt'a ekle
		prompt := fmt.Sprintf("Şu an kubectl context: %s\n\nİstek: %s", m.currentCtx, query)
		
		raw, err := m.aiProvider.Complete(ctx, ai.K8sSystemPrompt, prompt)
		if err != nil {
			return aiResponseMsg{err: err}
		}

		// JSON parse et
		raw = strings.TrimSpace(raw)
		// Bazen model ```json ``` sarmalıyla döner, temizle
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)

		var response AIResponse
		if err := json.Unmarshal([]byte(raw), &response); err != nil {
			return aiResponseMsg{err: fmt.Errorf("AI yanıtı parse edilemedi: %w\n\nYanıt: %s", err, raw)}
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

// ─── Render Fonksiyonları ─────────────────────────────────────────────────────

func (m *Model) renderResults(resp *AIResponse, results []*k8s.CommandResult) {
	var sb strings.Builder

	sb.WriteString(labelStyle.Render("📋 Açıklama") + "\n")
	sb.WriteString(resp.Explanation + "\n\n")

	for _, result := range results {
		sb.WriteString(labelStyle.Render("⚡ Komut") + "\n")
		sb.WriteString(commandStyle.Render(result.Command) + "\n\n")

		if result.ExitCode != 0 {
			sb.WriteString(errorStyle.Render("✗ Hata (exit code: "+fmt.Sprintf("%d", result.ExitCode)+")") + "\n")
			if result.Stderr != "" {
				sb.WriteString(result.Stderr + "\n")
			}
		} else {
			sb.WriteString(labelStyle.Render("📊 Sonuç") + "\n")
			if result.Stdout != "" {
				sb.WriteString(outputStyle.Render(result.Stdout) + "\n")
			} else {
				sb.WriteString("(boş çıktı)\n")
			}
		}

		sb.WriteString(fmt.Sprintf("⏱ %s\n\n", result.Duration.Round(time.Millisecond)))
	}

	sb.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render("Ctrl+N: yeni sorgu  •  Ctrl+D: çıkış"))

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
	sb.WriteString("\n" + warningStyle.Render("Enter = onaylıyorum  •  Esc = iptal"))
	m.viewport.SetContent(sb.String())
}

func (m *Model) renderError(err error) {
	m.viewport.SetContent(errorStyle.Render("✗ Hata: "+err.Error()) +
		"\n\n" + lipgloss.NewStyle().Faint(true).Render("Ctrl+N: tekrar dene"))
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m *Model) View() string {
	var sections []string

	// Header
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render("⎈ kam"),
		lipgloss.NewStyle().Faint(true).Padding(0, 1).Render("ctx: "+m.currentCtx),
		lipgloss.NewStyle().Faint(true).Render("| "+m.aiProvider.Name()),
	)
	sections = append(sections, header)

	// İçerik alanı
	switch m.state {
	case stateInput:
		sections = append(sections, inputStyle.Render(m.input.View()))
		sections = append(sections, lipgloss.NewStyle().Faint(true).Render("Alt+Enter gönder  •  Ctrl+D çıkış"))
		sections = append(sections, m.viewport.View())

	case stateThinking:
		sections = append(sections, inputStyle.Render(m.input.View()))
		sections = append(sections, m.spinner.View()+" AI düşünüyor...")

	case stateConfirm:
		sections = append(sections, m.viewport.View())

	case stateExecuting:
		sections = append(sections, m.spinner.View()+" Komutlar çalıştırılıyor...")

	case stateResult:
		sections = append(sections, m.viewport.View())
		sections = append(sections, lipgloss.NewStyle().Faint(true).Render("Ctrl+N: yeni sorgu"))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
