package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	timeout time.Duration
	http    *http.Client
}

type InferRequest struct {
	Text         string  `json:"text"`
	ModelVersion string  `json:"model_version"`
	Threshold    float64 `json:"threshold"`
}

type Explanation struct {
	KeyPhrases   []string `json:"key_phrases"`
	TopSentences []string `json:"top_sentences"`
}

type InferResponse struct {
	Label       string      `json:"label"`
	Score       float64     `json:"score"`
	Confidence  float64     `json:"confidence"`
	Explanation Explanation `json:"explanation"`
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &Client{
		baseURL: baseURL,
		timeout: timeout,
		http: &http.Client{
			Timeout:   timeout + 1*time.Second,
			Transport: transport,
		},
	}
}

func (c *Client) Infer(ctx context.Context, req InferRequest) (InferResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	body, err := json.Marshal(req)
	if err != nil {
		return InferResponse{}, fmt.Errorf("marshal infer request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/infer", bytes.NewReader(body))
	if err != nil {
		return InferResponse{}, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return InferResponse{}, fmt.Errorf("ai request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return InferResponse{}, fmt.Errorf("read ai response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return InferResponse{}, fmt.Errorf("ai status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var out InferResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return InferResponse{}, fmt.Errorf("unmarshal ai response: %w", err)
	}
	return out, nil
}
