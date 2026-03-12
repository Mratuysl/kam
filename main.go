package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/yourusername/kam/ai"
	"github.com/yourusername/kam/config"
	"github.com/yourusername/kam/k8s"
	"github.com/yourusername/kam/tui"
)

var rootCmd = &cobra.Command{
	Use:   "kam",
	Short: "⎈ kam — Doğal dille Kubernetes yönetimi",
	Long: `kai, doğal dil komutlarını kubectl'e çeviren AI destekli 
bir Kubernetes terminal aracıdır.

Örnek kullanım:
  kai                          # TUI başlat
  kai ask "prod'daki yüksek memory kullanan podlar"
  kai config set provider claude`,
	RunE: runTUI,
}

var askCmd = &cobra.Command{
	Use:   "ask [sorgu]",
	Short: "Tek seferlik sorgu yap ve çık",
	Args:  cobra.ExactArgs(1),
	RunE:  runAsk,
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "kai yapılandırmasını yönet",
}

var configSetCmd = &cobra.Command{
	Use:   "set [alan] [değer]",
	Short: "Config değeri ayarla (örn: kai config set provider openai)",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

func init() {
	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetCmd)

	// Global flaglar
	rootCmd.PersistentFlags().String("provider", "", "AI provider (claude, openai, ollama)")
	rootCmd.PersistentFlags().String("model", "", "Model adı")
	rootCmd.PersistentFlags().String("kubeconfig", "", "kubeconfig dosya yolu")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ─── TUI Modu ─────────────────────────────────────────────────────────────────

func runTUI(cmd *cobra.Command, args []string) error {
	cfg, provider, client, err := setup(cmd)
	if err != nil {
		return err
	}
	_ = cfg

	m := tui.New(provider, client)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI hatası: %w", err)
	}
	return nil
}

// ─── Tek Sorgu Modu ───────────────────────────────────────────────────────────

func runAsk(cmd *cobra.Command, args []string) error {
	_, provider, client, err := setup(cmd)
	if err != nil {
		return err
	}

	query := args[0]
	ctx := cmd.Context()

	// AI'a sor
	fmt.Printf("🤔 Sorgu: %s\n\n", query)
	raw, err := provider.Complete(ctx, ai.K8sSystemPrompt, query)
	if err != nil {
		return err
	}

	fmt.Printf("AI yanıtı:\n%s\n", raw)
	_ = client
	return nil
}

// ─── Config Ayarla ────────────────────────────────────────────────────────────

func runConfigSet(cmd *cobra.Command, args []string) error {
	field, value := args[0], args[1]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch field {
	case "provider":
		cfg.AI.Provider = config.AIProvider(value)
		fmt.Printf("✓ Provider '%s' olarak ayarlandı\n", value)
	case "model":
		cfg.AI.Model = value
		fmt.Printf("✓ Model '%s' olarak ayarlandı\n", value)
	case "api_key":
		cfg.AI.APIKey = value
		fmt.Printf("✓ API key kaydedildi\n")
	case "name":
		cfg.UI.AgentName = value
		fmt.Printf("✓ Ajan adı '%s' olarak ayarlandı\n", value)
		fmt.Printf("  Artık '%s ask \"...\"' ile kullanabilirsin\n", value)
	default:
		return fmt.Errorf("bilinmeyen alan: %s (provider, model, api_key, name)", field)
	}

	return config.Save(cfg)
}

// ─── Ortak Setup ──────────────────────────────────────────────────────────────

func setup(cmd *cobra.Command) (*config.Config, ai.Provider, *k8s.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("config yüklenemedi: %w", err)
	}

	// Flag override'ları
	if v, _ := cmd.Flags().GetString("provider"); v != "" {
		cfg.AI.Provider = config.AIProvider(v)
	}
	if v, _ := cmd.Flags().GetString("model"); v != "" {
		cfg.AI.Model = v
	}
	if v, _ := cmd.Flags().GetString("kubeconfig"); v != "" {
		cfg.Kubernetes.KubeconfigPath = v
	}

	// AI Provider başlat
	provider, err := ai.New(cfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("AI provider başlatılamadı: %w\n💡 İpucu: ANTHROPIC_API_KEY env var'ını kontrol et", err)
	}

	// Kubernetes Client başlat
	client := k8s.New(cfg.Kubernetes.KubeconfigPath)

	return cfg, provider, client, nil
}
