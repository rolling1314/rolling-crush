package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Cloudflare API Token
const cfToken = "HBMEL2SWzLuqEE-hw-ccj4YjCLil6Bbx7LclzOOi"

// 域名
const domain = "rollingcoding.com"

// 三级域名和目标 IP
var subdomain = "api.rollingcoding.com"
var targetIP = "1.2.3.4"

// 结构体
type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DNSRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

type CFResponse struct {
	Success bool            `json:"success"`
	Errors  []interface{}   `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

// 获取 Zone ID
func getZoneID() (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", domain)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var r struct {
		Success bool   `json:"success"`
		Result  []Zone `json:"result"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	if !r.Success || len(r.Result) == 0 {
		return "", fmt.Errorf("failed to get zone id")
	}
	return r.Result[0].ID, nil
}

// 查找是否存在 DNS 记录
func getDNSRecordID(zoneID, name string) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=A&name=%s", zoneID, name)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Success bool         `json:"success"`
		Result  []DNSRecord  `json:"result"`
		Errors  []interface{} `json:"errors"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	if len(r.Result) > 0 {
		return r.Result[0].ID, nil
	}
	return "", nil
}

// 创建 DNS 记录
func createDNSRecord(zoneID string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)
	record := DNSRecord{
		Type:    "A",
		Name:    subdomain,
		Content: targetIP,
		TTL:     120,
		Proxied: false,
	}
	data, _ := json.Marshal(record)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+cfToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("Create response:", string(body))
	return nil
}

// 更新 DNS 记录
func updateDNSRecord(zoneID, recordID string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, recordID)
	record := DNSRecord{
		Type:    "A",
		Name:    subdomain,
		Content: targetIP,
		TTL:     120,
		Proxied: false,
	}
	data, _ := json.Marshal(record)

	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+cfToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("Update response:", string(body))
	return nil
}

func main() {
	zoneID, err := getZoneID()
	if err != nil {
		fmt.Println("Error getting zone ID:", err)
		return
	}

	recordID, err := getDNSRecordID(zoneID, subdomain)
	if err != nil {
		fmt.Println("Error checking DNS record:", err)
		return
	}

	if recordID == "" {
		fmt.Println("DNS record not found, creating...")
		if err := createDNSRecord(zoneID); err != nil {
			fmt.Println("Create failed:", err)
		}
	} else {
		fmt.Println("DNS record exists, updating...")
		if err := updateDNSRecord(zoneID, recordID); err != nil {
			fmt.Println("Update failed:", err)
		}
	}
}
