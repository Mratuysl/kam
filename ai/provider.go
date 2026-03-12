package ai

import (
	"context"
	"fmt"

	"github.com/Mratuysl/kam/config"
)

// Provider tüm AI sağlayıcılarının uygulaması gereken interface
type Provider interface {
	Complete(ctx context.Context, system, prompt string) (string, error)
	Name() string
}

// K8sSystemPrompt — AI'ya Kubernetes uzmanı rolünü ver
const K8sSystemPrompt = `Sen bir Kubernetes uzmanısın. SADECE aşağıdaki JSON formatında yanıt vermelisin. Başka hiçbir şey yazma, açıklama yapma, markdown kullanma.

ZORUNLU FORMAT:
{"commands":["kubectl komut1","kubectl komut2"],"explanation":"ne yapacağının kısa açıklaması","warning":"güvenli mi değil mi","dangerous":false}

ÖRNEK:
Kullanıcı: "tüm podları listele"
Yanıt: {"commands":["kubectl get pods --all-namespaces"],"explanation":"Tüm namespace'lerdeki podları listeler","warning":"Sadece okuma, güvenli","dangerous":false}

Tehlikeli operasyonlarda (delete, drain, cordon, scale to 0):
{"commands":["kubectl delete ..."],"explanation":"...","warning":"DİKKAT: Bu işlem geri alınamaz","dangerous":true}

KURAL: Yanıtın ilk karakteri { ve son karakteri } olmalı. Başka hiçbir şey yazma.`

// New config'e göre doğru provider'ı döndürür
func New(cfg *config.Config) (Provider, error) {
	switch cfg.AI.Provider {
	case config.ProviderClaude:
		return NewClaude(cfg)
	case config.ProviderOpenAI:
		return NewOpenAI(cfg)
	case config.ProviderOllama:
		return NewOllama(cfg)
    case config.ProviderReplicate:
        return NewReplicate(cfg)
	default:
		return nil, fmt.Errorf("bilinmeyen AI provider: %s", cfg.AI.Provider)
	}
}
