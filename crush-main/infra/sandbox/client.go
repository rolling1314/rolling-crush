package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client 沙箱服务HTTP客户端
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient 创建沙箱客户端
func NewClient(baseURL string) *Client {
	return &Client{
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
func (c *Client) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
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
func (c *Client) ReadFile(ctx context.Context, req FileReadRequest) (*FileReadResponse, error) {
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
func (c *Client) WriteFile(ctx context.Context, req FileWriteRequest) (*FileWriteResponse, error) {
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
func (c *Client) ListFiles(ctx context.Context, req FileListRequest) (*FileListResponse, error) {
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
func (c *Client) Grep(ctx context.Context, req GrepRequest) (*GrepResponse, error) {
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
func (c *Client) Glob(ctx context.Context, req GlobRequest) (*GlobResponse, error) {
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
func (c *Client) EditFile(ctx context.Context, req FileEditRequest) (*FileEditResponse, error) {
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

// FileTreeRequest 获取文件树请求
type FileTreeRequest struct {
	SessionID string `json:"session_id,omitempty"` // 通过会话ID获取（向后兼容）
	ProjectID string `json:"project_id,omitempty"` // 通过项目ID获取（推荐）
	Path      string `json:"path,omitempty"`
}

// FileNode 文件节点
type FileNode struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Type     string     `json:"type"` // "file" 或 "folder"
	Path     string     `json:"path"`
	Content  string     `json:"content,omitempty"`
	Children []FileNode `json:"children,omitempty"`
}

// FileTreeResponse 获取文件树响应
type FileTreeResponse struct {
	Status string   `json:"status"`
	Tree   FileNode `json:"tree"`
	Error  string   `json:"error,omitempty"`
}

// GetFileTree 获取文件树
func (c *Client) GetFileTree(ctx context.Context, req FileTreeRequest) (*FileTreeResponse, error) {
	// 构建 URL with query parameters
	// 优先使用 ProjectID（新方式），否则使用 SessionID（向后兼容）
	var url string
	if req.ProjectID != "" {
		url = fmt.Sprintf("%s/file/tree?project_id=%s", c.baseURL, req.ProjectID)
	} else if req.SessionID != "" {
		url = fmt.Sprintf("%s/file/tree?session_id=%s", c.baseURL, req.SessionID)
	} else {
		return nil, fmt.Errorf("either SessionID or ProjectID must be provided")
	}

	if req.Path != "" {
		url = fmt.Sprintf("%s&path=%s", url, req.Path)
	}

	fmt.Printf("📤 Sandbox: GET %s\n", url)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		fmt.Printf("❌ Sandbox: 创建请求失败: %v\n", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		fmt.Printf("❌ Sandbox: 发送请求失败: %v\n", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

	respData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		fmt.Printf("❌ Sandbox: 读取响应失败: %v\n", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("📥 Sandbox: 状态码 %d, 响应大小 %d 字节\n", httpResp.StatusCode, len(respData))

	if httpResp.StatusCode != http.StatusOK {
		fmt.Printf("❌ Sandbox: 错误状态码 %d: %s\n", httpResp.StatusCode, string(respData))
		return nil, fmt.Errorf("sandbox returned status %d: %s", httpResp.StatusCode, string(respData))
	}

	var resp FileTreeResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		fmt.Printf("❌ Sandbox: 解析响应失败: %v\n", err)
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.Error != "" {
		return &resp, fmt.Errorf("sandbox error: %s", resp.Error)
	}

	fmt.Printf("✅ Sandbox: 请求成功\n")
	return &resp, nil
}

// doRequest 通用HTTP请求方法
func (c *Client) doRequest(ctx context.Context, method, path string, reqBody, respBody interface{}) error {
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

// CreateProjectRequest 创建项目请求
type CreateProjectRequest struct {
	ProjectName     string `json:"project_name"`
	BackendLanguage string `json:"backend_language,omitempty"` // "", "go", "java", "python"
	NeedDatabase    bool   `json:"need_database"`
}

// CreateProjectResponse 创建项目响应
type CreateProjectResponse struct {
	Status        string `json:"status"`
	ContainerID   string `json:"container_id"`   // 容器ID (12位短ID)
	ContainerName string `json:"container_name"` // 容器名称
	FrontendPort  int32  `json:"frontend_port"`
	BackendPort   *int32 `json:"backend_port,omitempty"`
	Image         string `json:"image"`
	Workdir       string `json:"workdir"` // 工作目录
	Message       string `json:"message"`
	Error         string `json:"error,omitempty"`
}

// CreateProject 创建项目容器
func (c *Client) CreateProject(ctx context.Context, req CreateProjectRequest) (*CreateProjectResponse, error) {
	var resp CreateProjectResponse
	err := c.doRequest(ctx, "POST", "/projects/create", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return &resp, fmt.Errorf("sandbox error: %s", resp.Error)
	}
	return &resp, nil
}

// DeleteProjectRequest 删除项目请求
type DeleteProjectRequest struct {
	ContainerID string `json:"container_id"`
}

// DeleteProjectResponse 删除项目响应
type DeleteProjectResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// DeleteProject 删除项目容器
func (c *Client) DeleteProject(ctx context.Context, req DeleteProjectRequest) (*DeleteProjectResponse, error) {
	var resp DeleteProjectResponse
	err := c.doRequest(ctx, "POST", "/projects/delete", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return &resp, fmt.Errorf("sandbox error: %s", resp.Error)
	}
	return &resp, nil
}

// ConfigureDomainRequest 配置域名请求
type ConfigureDomainRequest struct {
	ContainerID  string `json:"container_id"`
	Subdomain    string `json:"subdomain"`     // 三级域名前缀，如 "abc1234567"
	FrontendPort int32  `json:"frontend_port"` // 主机端口
	Domain       string `json:"domain"`        // 基础域名，如 "rollingcoding.com"
}

// ConfigureDomainResponse 配置域名响应
type ConfigureDomainResponse struct {
	Status    string `json:"status"`
	Subdomain string `json:"subdomain"` // 完整三级域名
	Message   string `json:"message"`
	Error     string `json:"error,omitempty"`
}

// ConfigureDomain 配置项目域名（nginx + vite）
func (c *Client) ConfigureDomain(ctx context.Context, req ConfigureDomainRequest) (*ConfigureDomainResponse, error) {
	var resp ConfigureDomainResponse
	err := c.doRequest(ctx, "POST", "/projects/configure-domain", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return &resp, fmt.Errorf("sandbox error: %s", resp.Error)
	}
	return &resp, nil
}

// ProjectStartupRequest 项目启动动作请求
// Command/Language/WorkingDir 为空时由 sandbox/config.yaml 配置决定。
type ProjectStartupRequest struct {
	ProjectID  string `json:"project_id"`
	Command    string `json:"command,omitempty"`
	Language   string `json:"language,omitempty"`
	WorkingDir string `json:"working_dir,omitempty"`
}

// ProjectStartupResponse 项目启动动作响应
type ProjectStartupResponse struct {
	Status   string `json:"status"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Command  string `json:"command,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ProjectStartup 触发项目容器启动动作
func (c *Client) ProjectStartup(ctx context.Context, req ProjectStartupRequest) (*ProjectStartupResponse, error) {
	var resp ProjectStartupResponse
	err := c.doRequest(ctx, "POST", "/projects/startup", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return &resp, fmt.Errorf("sandbox error: %s", resp.Error)
	}
	return &resp, nil
}

// GetDefaultClient 获取默认的沙箱客户端（单例）
var defaultClient *Client

func GetDefaultClient() *Client {
	if defaultClient == nil {
		// 从配置文件获取沙箱服务地址
		// 注意：需要在应用启动时先初始化配置
		// 如果配置未初始化，将使用默认值
		baseURL := "http://localhost:8888" // 默认值

		// 尝试导入配置包（避免循环依赖）
		// 实际使用时应该通过依赖注入传入配置
		defaultClient = NewClient(baseURL)
	}
	return defaultClient
}

// SetDefaultClient 设置默认的沙箱客户端
func SetDefaultClient(baseURL string) {
	defaultClient = NewClient(baseURL)
}

// ==================== LSP 诊断相关 ====================

// DiagnosticSeverity LSP诊断严重程度
type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

// DiagnosticTag LSP诊断标签
type DiagnosticTag int

const (
	TagUnnecessary DiagnosticTag = 1
	TagDeprecated  DiagnosticTag = 2
)

// Position LSP位置
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range LSP范围
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Diagnostic LSP诊断信息
type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity"`
	Code     interface{}        `json:"code,omitempty"`
	Source   string             `json:"source,omitempty"`
	Message  string             `json:"message"`
	Tags     []DiagnosticTag    `json:"tags,omitempty"`
}

// FileDiagnostics 单个文件的诊断信息
type FileDiagnostics struct {
	FilePath    string       `json:"file_path"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// LSPDiagnosticsRequest LSP诊断请求
type LSPDiagnosticsRequest struct {
	SessionID string `json:"session_id"`
	FilePath  string `json:"file_path,omitempty"` // 可选，为空则返回项目级诊断
}

// LSPDiagnosticsResponse LSP诊断响应
type LSPDiagnosticsResponse struct {
	Status             string            `json:"status"`
	FileDiagnostics    []FileDiagnostics `json:"file_diagnostics"`              // 当前文件的诊断
	ProjectDiagnostics []FileDiagnostics `json:"project_diagnostics,omitempty"` // 项目级诊断
	Error              string            `json:"error,omitempty"`
}

// GetLSPDiagnostics 获取LSP诊断信息
func (c *Client) GetLSPDiagnostics(ctx context.Context, req LSPDiagnosticsRequest) (*LSPDiagnosticsResponse, error) {
	var resp LSPDiagnosticsResponse
	err := c.doRequest(ctx, "POST", "/lsp/diagnostics", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return &resp, fmt.Errorf("sandbox error: %s", resp.Error)
	}
	return &resp, nil
}
