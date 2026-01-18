package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client Cloudflare DNS API 客户端
type Client struct {
	apiToken   string
	domain     string
	httpClient *http.Client
	zoneID     string // 缓存的 Zone ID
}

// NewClient 创建 Cloudflare 客户端
func NewClient(apiToken, domain string) *Client {
	return &Client{
		apiToken: apiToken,
		domain:   domain,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Zone Cloudflare Zone 信息
type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DNSRecord DNS 记录
type DNSRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// cfResponse Cloudflare API 通用响应
type cfResponse struct {
	Success bool            `json:"success"`
	Errors  []interface{}   `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

// GetZoneID 获取域名的 Zone ID
func (c *Client) GetZoneID(ctx context.Context) (string, error) {
	// 如果已经缓存了 Zone ID，直接返回
	if c.zoneID != "" {
		return c.zoneID, nil
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", c.domain)
	fmt.Printf("📤 Cloudflare: GET %s\n", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Printf("❌ Cloudflare: Request failed: %v\n", err)
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("📥 Cloudflare: Status %d, Response: %s\n", resp.StatusCode, string(body))

	var r struct {
		Success bool          `json:"success"`
		Result  []Zone        `json:"result"`
		Errors  []interface{} `json:"errors"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if !r.Success {
		fmt.Printf("❌ Cloudflare API errors: %v\n", r.Errors)
		return "", fmt.Errorf("cloudflare API error: %v", r.Errors)
	}

	if len(r.Result) == 0 {
		fmt.Printf("❌ Cloudflare: No zones found for domain %s\n", c.domain)
		return "", fmt.Errorf("no zones found for domain %s", c.domain)
	}

	c.zoneID = r.Result[0].ID
	fmt.Printf("✅ Cloudflare: Zone ID = %s\n", c.zoneID)
	return c.zoneID, nil
}

// GetDNSRecordID 查找是否存在指定的 DNS 记录
func (c *Client) GetDNSRecordID(ctx context.Context, name string) (string, error) {
	zoneID, err := c.GetZoneID(ctx)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=A&name=%s", zoneID, name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var r struct {
		Success bool        `json:"success"`
		Result  []DNSRecord `json:"result"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(r.Result) > 0 {
		return r.Result[0].ID, nil
	}
	return "", nil
}

// CreateDNSRecord 创建 DNS A 记录
func (c *Client) CreateDNSRecord(ctx context.Context, subdomain, targetIP string) error {
	zoneID, err := c.GetZoneID(ctx)
	if err != nil {
		return err
	}

	fullDomain := fmt.Sprintf("%s.%s", subdomain, c.domain)
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)

	record := DNSRecord{
		Type:    "A",
		Name:    fullDomain,
		Content: targetIP,
		TTL:     120,
		Proxied: false,
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var r cfResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !r.Success {
		return fmt.Errorf("cloudflare API error: %v", r.Errors)
	}

	fmt.Printf("✅ Cloudflare: DNS record created for %s -> %s\n", fullDomain, targetIP)
	return nil
}

// UpdateDNSRecord 更新 DNS A 记录
func (c *Client) UpdateDNSRecord(ctx context.Context, recordID, subdomain, targetIP string) error {
	zoneID, err := c.GetZoneID(ctx)
	if err != nil {
		return err
	}

	fullDomain := fmt.Sprintf("%s.%s", subdomain, c.domain)
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, recordID)

	record := DNSRecord{
		Type:    "A",
		Name:    fullDomain,
		Content: targetIP,
		TTL:     120,
		Proxied: false,
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var r cfResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !r.Success {
		return fmt.Errorf("cloudflare API error: %v", r.Errors)
	}

	fmt.Printf("✅ Cloudflare: DNS record updated for %s -> %s\n", fullDomain, targetIP)
	return nil
}

// AddOrUpdateDNSRecord 添加或更新 DNS 记录
func (c *Client) AddOrUpdateDNSRecord(ctx context.Context, subdomain, targetIP string) error {
	fullDomain := fmt.Sprintf("%s.%s", subdomain, c.domain)

	// 检查记录是否存在
	recordID, err := c.GetDNSRecordID(ctx, fullDomain)
	if err != nil {
		return fmt.Errorf("failed to check DNS record: %w", err)
	}

	if recordID == "" {
		// 记录不存在，创建新记录
		fmt.Printf("📝 Cloudflare: Creating DNS record for %s\n", fullDomain)
		return c.CreateDNSRecord(ctx, subdomain, targetIP)
	}

	// 记录存在，更新记录
	fmt.Printf("📝 Cloudflare: Updating DNS record for %s\n", fullDomain)
	return c.UpdateDNSRecord(ctx, recordID, subdomain, targetIP)
}

// GetDomain 获取基础域名
func (c *Client) GetDomain() string {
	return c.domain
}
