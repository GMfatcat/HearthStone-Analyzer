package settings

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	sqliteStore "hearthstone-analyzer/internal/storage/sqlite"
)

const (
	KeyLLMAPIKey  = "llm.api_key"
	KeyLLMBaseURL = "llm.base_url"
	KeyLLMModel   = "llm.model"
)

var catalog = map[string]Definition{
	KeyLLMAPIKey: {
		Key:         KeyLLMAPIKey,
		Sensitive:   true,
		Description: "API key for the configured OpenAI-compatible endpoint.",
	},
	KeyLLMBaseURL: {
		Key:         KeyLLMBaseURL,
		Sensitive:   false,
		Description: "Base URL for the configured OpenAI-compatible endpoint.",
	},
	KeyLLMModel: {
		Key:         KeyLLMModel,
		Sensitive:   false,
		Description: "Default model name used for AI report generation.",
	},
}

type Definition struct {
	Key         string
	Sensitive   bool
	Description string
}

type Input struct {
	Key   string
	Value string
}

type Setting struct {
	Key         string
	Value       string
	Sensitive   bool
	Description string
}

type Codec interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

type Service struct {
	repo  *sqliteStore.SettingsRepository
	codec Codec
}

type AESGCMCodec struct {
	aead cipher.AEAD
}

func NewService(repo *sqliteStore.SettingsRepository, codec Codec) *Service {
	return &Service{
		repo:  repo,
		codec: codec,
	}
}

func NewAESGCMCodec(key string) (*AESGCMCodec, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: got %d, want 32", len(key))
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create aes-gcm: %w", err)
	}

	return &AESGCMCodec{aead: aead}, nil
}

func (c *AESGCMCodec) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}

	ciphertext := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

func (c *AESGCMCodec) Decrypt(ciphertext string) (string, error) {
	payload, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	nonceSize := c.aead.NonceSize()
	if len(payload) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce := payload[:nonceSize]
	encrypted := payload[nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext: %w", err)
	}

	return string(plaintext), nil
}

func (s *Service) Upsert(ctx context.Context, input Input) error {
	def, err := LookupDefinition(input.Key)
	if err != nil {
		return err
	}

	value := strings.TrimSpace(input.Value)
	if value == "" {
		return fmt.Errorf("setting %q cannot be empty", input.Key)
	}

	storedValue := value
	isEncrypted := false
	if def.Sensitive {
		if s.codec == nil {
			return fmt.Errorf("setting %q requires encryption codec", input.Key)
		}

		storedValue, err = s.codec.Encrypt(value)
		if err != nil {
			return fmt.Errorf("encrypt setting %q: %w", input.Key, err)
		}
		isEncrypted = true
	}

	return s.repo.Upsert(ctx, sqliteStore.Setting{
		Key:         input.Key,
		Value:       storedValue,
		IsEncrypted: isEncrypted,
	})
}

func (s *Service) Get(ctx context.Context, key string) (Setting, error) {
	def, err := LookupDefinition(key)
	if err != nil {
		return Setting{}, err
	}

	raw, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return Setting{}, err
	}

	value, err := s.decode(raw.Value, raw.IsEncrypted)
	if err != nil {
		return Setting{}, fmt.Errorf("decode setting %q: %w", key, err)
	}

	return Setting{
		Key:         raw.Key,
		Value:       value,
		Sensitive:   def.Sensitive,
		Description: def.Description,
	}, nil
}

func (s *Service) List(ctx context.Context) ([]Setting, error) {
	rawSettings, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	rawByKey := make(map[string]sqliteStore.Setting, len(rawSettings))
	for _, raw := range rawSettings {
		rawByKey[raw.Key] = raw
	}

	keys := make([]string, 0, len(catalog))
	for key := range catalog {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	settings := make([]Setting, 0, len(keys))
	for _, key := range keys {
		def := catalog[key]
		value := ""
		if raw, ok := rawByKey[key]; ok {
			value, err = s.decode(raw.Value, raw.IsEncrypted)
			if err != nil {
				return nil, fmt.Errorf("decode setting %q: %w", raw.Key, err)
			}
		}

		settings = append(settings, Setting{
			Key:         key,
			Value:       value,
			Sensitive:   def.Sensitive,
			Description: def.Description,
		})
	}

	return settings, nil
}

func LookupDefinition(key string) (Definition, error) {
	def, ok := catalog[key]
	if !ok {
		return Definition{}, fmt.Errorf("unknown setting key %q", key)
	}

	return def, nil
}

func (s *Service) decode(value string, encrypted bool) (string, error) {
	if !encrypted {
		return value, nil
	}

	if s.codec == nil {
		return "", errors.New("missing codec for encrypted setting")
	}

	return s.codec.Decrypt(value)
}
