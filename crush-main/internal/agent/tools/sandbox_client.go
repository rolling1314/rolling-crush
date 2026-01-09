package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SandboxClient 沙箱服务HTTP客户端
type SandboxClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewSandboxClient 创建沙箱客户端
func NewSandboxClient(baseURL string) *SandboxClient {
	return &SandboxClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // 5分钟超时，适合长时间运行的命令
		},
	}
}

// ExecuteRequest 执行命令请求
type ExecuteRequest struct {
	SessionID  string `json:"session_id"`
	Command    string `json:"command"`
	Language   string `json:"language,omitempty"`
	WorkingDir string `json:"working_dir,omitempty"`
}

// ExecuteResponse 执行命令响应
type ExecuteResponse struct {
	Status   string `json:"status"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// FileReadRequest 读取文件请求
type FileReadRequest struct {
	SessionID string `json:"session_id"`
	FilePath  string `json:"file_path"`
}

// FileReadResponse 读取文件响应
type FileReadResponse struct {
	Status  string `json:"status"`
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// FileWriteRequest 写入文件请求
type FileWriteRequest struct {
	SessionID string `json:"session_id"`
	FilePath  string `json:"file_path"`
	Content   string `json:"content"`
}

// FileWriteResponse 写入文件响应
type FileWriteResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// FileListRequest 列出文件请求
type FileListRequest struct {
	SessionID string `json:"session_id"`
	Path      string `json:"path,omitempty"`
}

// FileListResponse 列出文件响应
type FileListResponse struct {
	Status string   `json:"status"`
	Files  []string `json:"files"`
	Error  string   `json:"error,omitempty"`
}

// GrepRequest 搜索文件内容请求
type GrepRequest struct {
	SessionID string `json:"session_id"`
	Pattern   string `json:"pattern"`
	Path      string `json:"path,omitempty"`
}

// GrepResponse 搜索文件内容响应
type GrepResponse struct {
	Status   string `json:"status"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// GlobRequest 文件名模式匹配请求
type GlobRequest struct {
	SessionID string `json:"session_id"`
	Pattern   string `json:"pattern"`
	Path      string `json:"path,omitempty"`
}

// GlobResponse 文件名模式匹配响应
type GlobResponse struct {
	Status   string `json:"status"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// FileEditRequest 编辑文件请求
type FileEditRequest struct {
	SessionID  string `json:"session_id"`
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

// FileEditResponse 编辑文件响应
type FileEditResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// Execute 在沙箱中执行命令
func (c *SandboxClient) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	var resp ExecuteResponse
	err := c.doRequest(ctx, "POST", "/execute", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return &resp, fmt.Errorf("sandbox error: %s", resp.Error)
	}
	return &resp, nil
}

// ReadFile 读取沙箱中的文件
func (c *SandboxClient) ReadFile(ctx context.Context, req FileReadRequest) (*FileReadResponse, error) {
	var resp FileReadResponse
	err := c.doRequest(ctx, "POST", "/file/read", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return &resp, fmt.Errorf("sandbox error: %s", resp.Error)
	}
	return &resp, nil
}

// WriteFile 写入文件到沙箱
func (c *SandboxClient) WriteFile(ctx context.Context, req FileWriteRequest) (*FileWriteResponse, error) {
	var resp FileWriteResponse
	err := c.doRequest(ctx, "POST", "/file/write", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return &resp, fmt.Errorf("sandbox error: %s", resp.Error)
	}
	return &resp, nil
}

// ListFiles 列出沙箱中的文件
func (c *SandboxClient) ListFiles(ctx context.Context, req FileListRequest) (*FileListResponse, error) {
	var resp FileListResponse
	err := c.doRequest(ctx, "POST", "/file/list", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return &resp, fmt.Errorf("sandbox error: %s", resp.Error)
	}
	return &resp, nil
}

// Grep 搜索文件内容
func (c *SandboxClient) Grep(ctx context.Context, req GrepRequest) (*GrepResponse, error) {
	var resp GrepResponse
	err := c.doRequest(ctx, "POST", "/file/grep", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return &resp, fmt.Errorf("sandbox error: %s", resp.Error)
	}
	return &resp, nil
}

// Glob 文件名模式匹配
func (c *SandboxClient) Glob(ctx context.Context, req GlobRequest) (*GlobResponse, error) {
	var resp GlobResponse
	err := c.doRequest(ctx, "POST", "/file/glob", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return &resp, fmt.Errorf("sandbox error: %s", resp.Error)
	}
	return &resp, nil
}

// EditFile 编辑文件内容
func (c *SandboxClient) EditFile(ctx context.Context, req FileEditRequest) (*FileEditResponse, error) {
	var resp FileEditResponse
	err := c.doRequest(ctx, "POST", "/file/edit", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return &resp, fmt.Errorf("sandbox error: %s", resp.Error)
	}
	return &resp, nil
}

// doRequest 通用HTTP请求方法
func (c *SandboxClient) doRequest(ctx context.Context, method, path string, reqBody, respBody interface{}) error {
	var body io.Reader
	var jsonData []byte
	if reqBody != nil {
		var err error
		jsonData, err = json.Marshal(reqBody)
		if err != nil {
			fmt.Printf("❌ Sandbox: Marshal 请求失败: %v (path: %s)\n", err, path)
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	url := c.baseURL + path
	
	// 打印请求信息
	fmt.Printf("📤 Sandbox: %s %s\n", method, url)
	if reqBody != nil && len(jsonData) < 500 {
		fmt.Printf("   请求体: %s\n", string(jsonData))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		fmt.Printf("❌ Sandbox: 创建请求失败: %v\n", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Printf("❌ Sandbox: 发送请求失败: %v\n", err)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ Sandbox: 读取响应失败: %v\n", err)
		return fmt.Errorf("failed to read response: %w", err)
	}

	// 打印响应信息
	fmt.Printf("📥 Sandbox: 状态码 %d, 响应大小 %d 字节\n", resp.StatusCode, len(respData))
	if len(respData) < 500 {
		fmt.Printf("   响应体: %s\n", string(respData))
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ Sandbox: 错误状态码 %d: %s\n", resp.StatusCode, string(respData))
		return fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(respData))
	}

	if respBody != nil {
		if err := json.Unmarshal(respData, respBody); err != nil {
			fmt.Printf("❌ Sandbox: 解析响应失败: %v\n", err)
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	fmt.Printf("✅ Sandbox: 请求成功\n")
	return nil
}

// GetDefaultSandboxClient 获取默认的沙箱客户端（单例）
var defaultSandboxClient *SandboxClient

func GetDefaultSandboxClient() *SandboxClient {
	if defaultSandboxClient == nil {
		// 默认连接到本地沙箱服务
		defaultSandboxClient = NewSandboxClient("http://localhost:8888")
	}
	return defaultSandboxClient
}
