package ai

import (
	"context"
	"fmt"

	"github.com/yourusername/kam/config"
)

// Provider tüm AI sağlayıcılarının uygulaması gereken interface
type Provider interface {
	Complete(ctx context.Context, system, prompt string) (string, error)
	Name() string
}

// K8sSystemPrompt — AI'ya Kubernetes uzmanı rolünü ver
const K8sSystemPrompt = `Sen bir Kubernetes uzmanısın. Görevin:

1. Kullanıcının doğal dil isteğini analiz et
2. Uygun kubectl komutunu veya komut zincirini üret
3. Komutun çıktısını nasıl yorumlaması gerektiğini açıkla

YANIT FORMATI (her zaman bu JSON formatında yanıt ver):
{
  "commands": ["kubectl get pods -n production", "kubectl top pods -n production"],
  "explanation": "Production namespace'indeki podları ve kaynak kullanımlarını listeleyecek",
  "warning": "Bu komut sadece okuma yapar, güvenli",
  "dangerous": false
}

Eğer istek tehlikeli bir operasyon içeriyorsa (delete, scale to 0, drain, cordon):
- dangerous: true yap
- warning alanında ne olacağını açıkça belirt

Sadece JSON yanıt ver, başka hiçbir şey yazma.`

// New config'e göre doğru provider'ı döndürür
func New(cfg *config.Config) (Provider, error) {
	switch cfg.AI.Provider {
	case config.ProviderClaude:
		return NewClaude(cfg)
	case config.ProviderOpenAI:
		return NewOpenAI(cfg)
	case config.ProviderOllama:
		return NewOllama(cfg)
	default:
		return nil, fmt.Errorf("bilinmeyen AI provider: %s", cfg.AI.Provider)
	}
}
