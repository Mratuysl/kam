package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CommandResult kubectl komutunun sonucunu tutar
type CommandResult struct {
	Stdout   string
	Stderr   string
	Duration time.Duration
	Command  string
	ExitCode int
}

// Client kubernetes işlemlerini yönetir
type Client struct {
	kubeconfigPath string
}

func New(kubeconfigPath string) *Client {
	return &Client{kubeconfigPath: kubeconfigPath}
}

// Run kubectl komutunu çalıştırır ve sonucu döndürür
func (c *Client) Run(ctx context.Context, command string) (*CommandResult, error) {
	// Güvenlik: sadece kubectl komutlarına izin ver
	if err := validateCommand(command); err != nil {
		return nil, err
	}

	parts := parseCommand(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("geçersiz komut")
	}

	start := time.Now()

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)

	// kubeconfig path'i override et (istenirse)
	if c.kubeconfigPath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", c.kubeconfigPath))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return &CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start),
		Command:  command,
		ExitCode: exitCode,
	}, nil
}

// RunMultiple birden fazla komutu sırayla çalıştırır
func (c *Client) RunMultiple(ctx context.Context, commands []string) ([]*CommandResult, error) {
	var results []*CommandResult
	for _, cmd := range commands {
		result, err := c.Run(ctx, cmd)
		if err != nil {
			return results, fmt.Errorf("komut başarısız '%s': %w", cmd, err)
		}
		results = append(results, result)
		// Bir komut hata verdiyse dur
		if result.ExitCode != 0 {
			break
		}
	}
	return results, nil
}

// GetContexts mevcut kubectl context'lerini listeler
func (c *Client) GetContexts(ctx context.Context) ([]string, error) {
	result, err := c.Run(ctx, "kubectl config get-contexts -o name")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	return lines, nil
}

// GetCurrentContext aktif context'i döndürür
func (c *Client) GetCurrentContext(ctx context.Context) (string, error) {
	result, err := c.Run(ctx, "kubectl config current-context")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

// validateCommand tehlikeli komutları engeller — AI onaylamadan çalışmasın
func validateCommand(command string) error {
	// Sadece kubectl komutuna izin ver
	trimmed := strings.TrimSpace(command)
	if !strings.HasPrefix(trimmed, "kubectl") {
		return fmt.Errorf("sadece kubectl komutlarına izin verilir")
	}

	// Shell injection engelle
	dangerousChars := []string{";", "&&", "||", "|", ">", "<", "`", "$"}
	for _, ch := range dangerousChars {
		if strings.Contains(trimmed, ch) {
			return fmt.Errorf("güvenlik: izin verilmeyen karakter '%s'", ch)
		}
	}

	return nil
}

// parseCommand komutu argümanlara böler
func parseCommand(command string) []string {
	return strings.Fields(command)
}

// IsDangerous komutun tehlikeli olup olmadığını kontrol eder
func IsDangerous(command string) bool {
	dangerous := []string{"delete", "drain", "cordon", "taint", "replace"}
	lower := strings.ToLower(command)
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return true
		}
	}
	return false
}
