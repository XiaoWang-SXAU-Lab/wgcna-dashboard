// Package ai implements the AI navigator: LLM provider configuration with
// encrypted-at-rest API key storage, an OpenAI-compatible chat client, and a
// local vector-retrieval knowledge base.
package ai

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// Provider is an LLM provider preset.
type Provider struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

// Providers are the built-in OpenAI-compatible presets.
var Providers = map[string]Provider{
	"deepseek":  {Name: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-chat"},
	"openai":    {Name: "OpenAI", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini"},
	"anthropic": {Name: "Anthropic", BaseURL: "https://api.anthropic.com/v1", Model: "claude-3-5-sonnet-latest"},
	"ollama":    {Name: "Ollama (local)", BaseURL: "http://localhost:11434/v1", Model: "llama3.1"},
}

// Config is the persisted LLM configuration. The API key is stored encrypted
// (APIKeyEnc), never in plaintext.
type Config struct {
	Provider  string `json:"provider"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKeyEnc string `json:"api_key_enc"`
}

// Store persists Config to a per-user config directory.
type Store struct {
	dir string
}

// NewStore returns a Store rooted at the per-user config directory.
func NewStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "wgcna-dashboard")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Load reads the persisted config (zero Config if none exists).
func (s *Store) Load() (Config, error) {
	var cfg Config
	b, err := os.ReadFile(filepath.Join(s.dir, "config.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Save persists the config, encrypting the API key at rest.
func (s *Store) Save(cfg Config, apiKey string) error {
	if apiKey != "" {
		key, err := s.masterKey()
		if err != nil {
			return err
		}
		enc, err := encrypt(apiKey, key)
		if err != nil {
			return err
		}
		cfg.APIKeyEnc = enc
	} else {
		cfg.APIKeyEnc = ""
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "config.json"), b, 0o600)
}

// APIKey decrypts the stored key. Returns "" if none is configured.
func (s *Store) APIKey(cfg Config) (string, error) {
	if cfg.APIKeyEnc == "" {
		return "", nil
	}
	key, err := s.masterKey()
	if err != nil {
		return "", err
	}
	return decrypt(cfg.APIKeyEnc, key)
}

// masterKey returns (or creates) the 256-bit machine-local encryption key.
func (s *Store) masterKey() ([]byte, error) {
	p := filepath.Join(s.dir, "secret.key")
	if b, err := os.ReadFile(p); err == nil && len(b) == 32 {
		return b, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func encrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

func decrypt(b64 string, key []byte) (string, error) {
	ct, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(ct) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, data := ct[:gcm.NonceSize()], ct[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
