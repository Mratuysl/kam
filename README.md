# ⎈ KAM — Kubernetes AI Manager

> Doğal dille Kubernetes yönetimi. k9s'in hızı + AI'ın zekası.

```
$ kam

⎈ kam  ctx: production-cluster  | Claude (claude-sonnet-4-20250514)

┌─────────────────────────────────────────────────────────────────┐
│ production namespace'indeki yüksek memory kullanan podları göster│
└─────────────────────────────────────────────────────────────────┘

📋 Açıklama
Production namespace'indeki podları memory kullanımına göre sıralayacak

⚡ Komut
 kubectl top pods -n production --sort-by=memory

📊 Sonuç
NAME                          CPU(cores)   MEMORY(bytes)
api-server-7d9f8b-xk2p9       245m         512Mi
worker-6c8b9f-mnp2k           89m          380Mi
redis-master-0                12m          256Mi

⏱ 1.2s
```

## Özellikler

- 🗣 **Doğal dil** — kubectl komutlarını ezberlemeye gerek yok
- 🔌 **Multi-provider** — Claude, OpenAI veya yerel Ollama
- 🛡 **Güvenli** — tehlikeli komutlar otomatik tespit, onay istenir
- 🎨 **Güzel çıktı** — renkli, formatlanmış sonuçlar
- 🏷 **Kendi adını ver** — ajanına istediğin ismi ver

## Kurulum

```bash
git clone https://github.com/yourusername/kam
cd kam
go mod tidy
go build -o kam .
sudo mv kam /usr/local/bin/
```

## Yapılandırma

```bash
export ANTHROPIC_API_KEY="sk-ant-..."

# Provider değiştir
kam config set provider openai
kam config set provider ollama   # ücretsiz, yerel

# Ajanına kendi adını ver
kam config set name jarvis
# artık: jarvis ask "prod'daki podlar"
```

Config: `~/.config/kam/config.yaml`

```yaml
ai:
  provider: claude
  model: claude-sonnet-4-20250514
  max_tokens: 2048
  ollama_url: http://localhost:11434
kubernetes:
  default_namespace: default
ui:
  theme: dark
  agent_name: kam
```

## Kullanım

```bash
kam                              # TUI aç
kam ask "restart eden podlar"    # hızlı sorgu
kam --provider ollama ask "..."  # anlık provider seç
```

| Tuş | Açıklama |
|-----|----------|
| `Alt+Enter` | Gönder |
| `Ctrl+N` | Yeni sorgu |
| `Enter` | Tehlikeli komut onayla |
| `Ctrl+D` | Çıkış |

## Güvenlik

- Sadece `kubectl` komutlarına izin verilir
- Shell injection karakterleri engellenir
- `delete`, `drain`, `cordon` otomatik tespit → onay gerekir

## Yol Haritası

- [ ] Konuşma geçmişi
- [ ] Multi-cluster desteği  
- [ ] Helm entegrasyonu
- [ ] Fine-tuned Kubernetes modeli

## Lisans

MIT
