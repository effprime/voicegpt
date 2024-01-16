package gptclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	RoleSystem    = "system"
	RoleAssistant = "assistant"
	RoleUser      = "user"
)

type GPTClient interface {
	Chat(*ChatCompletionRequest) (*ChatCompletionResponse, error)
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Choice struct {
	FinishReason string  `json:"finish_reason"`
	Index        int     `json:"index"`
	Message      Message `json:"message"`
}

type Usage struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type ChatCompletionResponse struct {
	Choices []Choice `json:"choices"`
	Created int64    `json:"created"`
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Object  string   `json:"object"`
	Usage   Usage    `json:"usage"`
}

type Client struct {
	URL        string
	HTTPClient *http.Client
	APIKey     string
}

func NewClient(apiKey string) GPTClient {
	return &Client{
		URL:        "https://api.openai.com/v1/chat/completions",
		HTTPClient: http.DefaultClient,
		APIKey:     apiKey,
	}
}

// Chat calls the ChatGPT/OpenAI API with the given completion request.
func (c *Client) Chat(req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", c.URL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Add("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	b, err = io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("received %v response from OpenAI API", httpResp.StatusCode)
	}

	resp := &ChatCompletionResponse{}
	err = json.Unmarshal(b, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
