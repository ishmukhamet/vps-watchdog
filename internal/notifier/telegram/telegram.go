package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ishmukhamet/vps-watchdog/internal/config"
)

const defaultBaseURL = "https://api.telegram.org"

type Client struct {
	Token     string
	ChatID    string
	BaseURL   string
	ParseMode string
	HTTP      *http.Client
}

func NewFromConfig(cfg config.TelegramConfig) (*Client, error) {
	token, err := readSecretFile(cfg.TokenFile)
	if err != nil {
		return nil, fmt.Errorf("read telegram token file: %w", err)
	}
	chatID, err := readSecretFile(cfg.ChatIDFile)
	if err != nil {
		return nil, fmt.Errorf("read telegram chat id file: %w", err)
	}
	if token == "" {
		return nil, fmt.Errorf("telegram token is empty")
	}
	if chatID == "" {
		return nil, fmt.Errorf("telegram chat id is empty")
	}
	return &Client{
		Token:     token,
		ChatID:    chatID,
		BaseURL:   defaultBaseURL,
		ParseMode: "HTML",
		HTTP:      &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (c *Client) Send(ctx context.Context, message string) error {
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("telegram token is empty")
	}
	if strings.TrimSpace(c.ChatID) == "" {
		return fmt.Errorf("telegram chat id is empty")
	}
	if c.BaseURL == "" {
		c.BaseURL = defaultBaseURL
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}

	form := url.Values{}
	form.Set("chat_id", c.ChatID)
	form.Set("text", message)
	if c.ParseMode != "" {
		form.Set("parse_mode", c.ParseMode)
	}
	form.Set("disable_web_page_preview", "true")

	endpoint := strings.TrimRight(c.BaseURL, "/") + "/bot" + c.Token + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("telegram request failed: %s", sanitizeTelegramText(err.Error(), c.Token))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram send failed: %s", sanitizeTelegramText(err.Error(), c.Token))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram send failed: http status %d: %s", resp.StatusCode, sanitizeTelegramBody(body, c.Token))
	}

	var parsed struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("telegram send failed: invalid response: %w", err)
	} else if !parsed.OK {
		return fmt.Errorf("telegram send failed: %s", sanitizeTelegramBody([]byte(parsed.Description), c.Token))
	}
	return nil
}

func readSecretFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func sanitizeTelegramBody(body []byte, token string) string {
	text := sanitizeTelegramText(string(body), token)
	if len(text) > 300 {
		text = text[:300] + "..."
	}
	return text
}

func sanitizeTelegramText(text, token string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	if token != "" {
		text = strings.ReplaceAll(text, token, "<redacted>")
	}
	return text
}
