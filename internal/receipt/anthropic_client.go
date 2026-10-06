package receipt

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/swiftahul20/expense-tracker/internal/category"
)

const (
	ProviderAnthropic = "anthropic"
	ProviderGemini    = "gemini"
)

type Client struct {
	APIKey     string
	HTTPClient *http.Client
	Provider   string
	Model      string
}

func NewClient(apiKey string) *Client {
	return &Client{APIKey: apiKey, HTTPClient: &http.Client{Timeout: 60 * time.Second}, Provider: ProviderAnthropic, Model: "claude-3-5-sonnet-20241022"}
}

func NewGeminiClient(apiKey string) *Client {
	return &Client{APIKey: apiKey, HTTPClient: &http.Client{Timeout: 60 * time.Second}, Provider: ProviderGemini, Model: "gemini-3.8-flash"}
}

func NewClientForProvider(provider, anthropicAPIKey, geminiAPIKey string) *Client {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case ProviderGemini:
		return NewGeminiClient(geminiAPIKey)
	default:
		return NewClient(anthropicAPIKey)
	}
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type   string                `json:"type"`
	Text   string                `json:"text,omitempty"`
	Source *anthropicImageSource `json:"source,omitempty"`
}

type anthropicImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) ScanImage(ctx context.Context, image []byte, mimeType string, categories []category.Category) (ScanResult, error) {
	if c == nil {
		return ScanResult{}, fmt.Errorf("%w: select a provider with LLM_PROVIDER", ErrNoAPIKey)
	}
	if c.APIKey == "" {
		keyName := "ANTHROPIC_API_KEY"
		if strings.EqualFold(strings.TrimSpace(c.Provider), ProviderGemini) {
			keyName = "GEMINI_API_KEY"
		}
		return ScanResult{}, fmt.Errorf("%w: set %s for the selected provider", ErrNoAPIKey, keyName)
	}
	if len(image) == 0 {
		return ScanResult{}, ErrInvalidImage
	}
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	switch strings.ToLower(strings.TrimSpace(c.Provider)) {
	case ProviderGemini:
		return c.scanWithGemini(ctx, image, mimeType, categories)
	default:
		return c.scanWithAnthropic(ctx, image, mimeType, categories)
	}
}

func (c *Client) scanWithAnthropic(ctx context.Context, image []byte, mimeType string, categories []category.Category) (ScanResult, error) {
	encoded := base64.StdEncoding.EncodeToString(image)
	body := anthropicRequest{
		Model:     c.Model,
		MaxTokens: 1024,
	}
	body.Messages = append(body.Messages, anthropicMessage{
		Role: "user",
		Content: []anthropicContent{
			{Type: "text", Text: BuildPrompt(categories)},
			{Type: "image", Source: &anthropicImageSource{Type: "base64", MediaType: mimeType, Data: encoded}},
		},
	})

	payload, err := json.Marshal(body)
	if err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	defer resp.Body.Close()

	var apiResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}

	if resp.StatusCode != http.StatusOK {
		message := apiResp.Error.Message
		if message == "" {
			message = "Anthropic API request failed"
		}
		return ScanResult{}, fmt.Errorf("%w: %s", ErrInvalidOutput, message)
	}

	var text strings.Builder
	for _, item := range apiResp.Content {
		if item.Type == "text" {
			text.WriteString(item.Text)
		}
	}
	if text.Len() == 0 {
		return ScanResult{}, fmt.Errorf("%w: empty response from Anthropic", ErrInvalidOutput)
	}

	result, err := ParseLLMResponse(text.String(), categories)
	if err != nil {
		return ScanResult{}, err
	}
	return result, nil
}

func (c *Client) scanWithGemini(ctx context.Context, image []byte, mimeType string, categories []category.Category) (ScanResult, error) {
	encoded := base64.StdEncoding.EncodeToString(image)
	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", c.Model, c.APIKey)

	body := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]any{
				{"text": BuildPrompt(categories)},
				{"inline_data": map[string]string{"mime_type": mimeType, "data": encoded}},
			},
		}},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	defer resp.Body.Close()

	var apiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}

	if resp.StatusCode != http.StatusOK {
		message := apiResp.Error.Message
		if message == "" {
			message = "Gemini API request failed"
		}
		return ScanResult{}, fmt.Errorf("%w: %s", ErrInvalidOutput, message)
	}

	var text strings.Builder
	for _, candidate := range apiResp.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				text.WriteString(part.Text)
			}
		}
	}
	if text.Len() == 0 {
		return ScanResult{}, fmt.Errorf("%w: empty response from Gemini", ErrInvalidOutput)
	}

	result, err := ParseLLMResponse(text.String(), categories)
	if err != nil {
		return ScanResult{}, err
	}
	return result, nil
}
