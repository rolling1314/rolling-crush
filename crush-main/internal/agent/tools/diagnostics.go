package tools

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/rolling1314/rolling-crush/internal/pkg/csync"
	"github.com/rolling1314/rolling-crush/internal/lsp"
	"github.com/rolling1314/rolling-crush/infra/sandbox"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

type DiagnosticsParams struct {
	FilePath string `json:"file_path,omitempty" description:"The path to the file to get diagnostics for (leave w empty for project diagnostics)"`
}

const DiagnosticsToolName = "lsp_diagnostics"

//go:embed diagnostics.md
var diagnosticsDescription []byte

func NewDiagnosticsTool(lspClients *csync.Map[string, *lsp.Client]) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		DiagnosticsToolName,
		string(diagnosticsDescription),
		func(ctx context.Context, params DiagnosticsParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// 使用沙箱诊断服务
			sessionID := GetSessionFromContext(ctx)
			if sessionID != "" {
				output := getSandboxDiagnostics(ctx, sessionID, params.FilePath)
				if output != "" {
					return fantasy.NewTextResponse(output), nil
				}
			}
			
			// 回退到本地 LSP 客户端（如果可用）
			if lspClients.Len() == 0 {
				return fantasy.NewTextErrorResponse("no LSP clients available"), nil
			}
			notifyLSPs(ctx, lspClients, params.FilePath)
			output := getDiagnostics(params.FilePath, lspClients)
			return fantasy.NewTextResponse(output), nil
		})
}

// notifyLSPsAndGetSandboxDiagnostics 通知并从沙箱获取诊断（用于 write/edit/multiedit 后）
func notifyLSPsAndGetSandboxDiagnostics(ctx context.Context, sessionID, filePath string) string {
	return getSandboxDiagnostics(ctx, sessionID, filePath)
}

// getSandboxDiagnostics 从沙箱获取诊断信息
func getSandboxDiagnostics(ctx context.Context, sessionID, filePath string) string {
	sandboxClient := sandbox.GetDefaultClient()
	
	resp, err := sandboxClient.GetLSPDiagnostics(ctx, sandbox.LSPDiagnosticsRequest{
		SessionID: sessionID,
		FilePath:  filePath,
	})
	
	if err != nil {
		slog.Warn("Failed to get sandbox diagnostics", "error", err)
		return ""
	}
	
	fileDiagnostics := []string{}
	projectDiagnostics := []string{}
	
	// 处理文件诊断
	for _, fd := range resp.FileDiagnostics {
		for _, diag := range fd.Diagnostics {
			formattedDiag := formatSandboxDiagnostic(fd.FilePath, diag)
			fileDiagnostics = append(fileDiagnostics, formattedDiag)
		}
	}
	
	// 处理项目诊断
	for _, pd := range resp.ProjectDiagnostics {
		for _, diag := range pd.Diagnostics {
			formattedDiag := formatSandboxDiagnostic(pd.FilePath, diag)
			projectDiagnostics = append(projectDiagnostics, formattedDiag)
		}
	}
	
	sortDiagnostics(fileDiagnostics)
	sortDiagnostics(projectDiagnostics)
	
	var output strings.Builder
	writeDiagnostics(&output, "file_diagnostics", fileDiagnostics)
	writeDiagnostics(&output, "project_diagnostics", projectDiagnostics)
	
	if len(fileDiagnostics) > 0 || len(projectDiagnostics) > 0 {
		fileErrors := countSeverity(fileDiagnostics, "Error")
		fileWarnings := countSeverity(fileDiagnostics, "Warn")
		projectErrors := countSeverity(projectDiagnostics, "Error")
		projectWarnings := countSeverity(projectDiagnostics, "Warn")
		output.WriteString("\n<diagnostic_summary>\n")
		fmt.Fprintf(&output, "Current file: %d errors, %d warnings\n", fileErrors, fileWarnings)
		fmt.Fprintf(&output, "Project: %d errors, %d warnings\n", projectErrors, projectWarnings)
		output.WriteString("</diagnostic_summary>\n")
	}
	
	out := output.String()
	if out != "" {
		slog.Info("Sandbox Diagnostics", "output", out)
	}
	return out
}

// formatSandboxDiagnostic 格式化沙箱诊断信息
func formatSandboxDiagnostic(path string, diagnostic sandbox.Diagnostic) string {
	severity := "Info"
	switch diagnostic.Severity {
	case sandbox.SeverityError:
		severity = "Error"
	case sandbox.SeverityWarning:
		severity = "Warn"
	case sandbox.SeverityHint:
		severity = "Hint"
	}
	
	location := fmt.Sprintf("%s:%d:%d", path, diagnostic.Range.Start.Line+1, diagnostic.Range.Start.Character+1)
	
	sourceInfo := diagnostic.Source
	if sourceInfo == "" {
		sourceInfo = "lsp"
	}
	
	codeInfo := ""
	if diagnostic.Code != nil {
		codeInfo = fmt.Sprintf("[%v]", diagnostic.Code)
	}
	
	tagsInfo := ""
	if len(diagnostic.Tags) > 0 {
		tags := []string{}
		for _, tag := range diagnostic.Tags {
			switch tag {
			case sandbox.TagUnnecessary:
				tags = append(tags, "unnecessary")
			case sandbox.TagDeprecated:
				tags = append(tags, "deprecated")
			}
		}
		if len(tags) > 0 {
			tagsInfo = fmt.Sprintf(" (%s)", strings.Join(tags, ", "))
		}
	}
	
	return fmt.Sprintf("%s: %s [%s]%s%s %s",
		severity,
		location,
		sourceInfo,
		codeInfo,
		tagsInfo,
		diagnostic.Message)
}

// ==================== 原有本地 LSP 相关函数（保留作为回退） ====================

func notifyLSPs(ctx context.Context, lsps *csync.Map[string, *lsp.Client], filepath string) {
	if filepath == "" {
		return
	}
	for client := range lsps.Seq() {
		if !client.HandlesFile(filepath) {
			continue
		}
		_ = client.OpenFileOnDemand(ctx, filepath)
		_ = client.NotifyChange(ctx, filepath)
		client.WaitForDiagnostics(ctx, 5*time.Second)
	}
}

func getDiagnostics(filePath string, lsps *csync.Map[string, *lsp.Client]) string {
	fileDiagnostics := []string{}
	projectDiagnostics := []string{}

	for lspName, client := range lsps.Seq2() {
		for location, diags := range client.GetDiagnostics() {
			path, err := location.Path()
			if err != nil {
				slog.Error("Failed to convert diagnostic location URI to path", "uri", location, "error", err)
				continue
			}
			isCurrentFile := path == filePath
			for _, diag := range diags {
				formattedDiag := formatDiagnostic(path, diag, lspName)
				if isCurrentFile {
					fileDiagnostics = append(fileDiagnostics, formattedDiag)
				} else {
					projectDiagnostics = append(projectDiagnostics, formattedDiag)
				}
			}
		}
	}

	sortDiagnostics(fileDiagnostics)
	sortDiagnostics(projectDiagnostics)

	var output strings.Builder
	writeDiagnostics(&output, "file_diagnostics", fileDiagnostics)
	writeDiagnostics(&output, "project_diagnostics", projectDiagnostics)

	if len(fileDiagnostics) > 0 || len(projectDiagnostics) > 0 {
		fileErrors := countSeverity(fileDiagnostics, "Error")
		fileWarnings := countSeverity(fileDiagnostics, "Warn")
		projectErrors := countSeverity(projectDiagnostics, "Error")
		projectWarnings := countSeverity(projectDiagnostics, "Warn")
		output.WriteString("\n<diagnostic_summary>\n")
		fmt.Fprintf(&output, "Current file: %d errors, %d warnings\n", fileErrors, fileWarnings)
		fmt.Fprintf(&output, "Project: %d errors, %d warnings\n", projectErrors, projectWarnings)
		output.WriteString("</diagnostic_summary>\n")
	}

	out := output.String()
	slog.Info("Diagnostics", "output", out)
	return out
}

func writeDiagnostics(output *strings.Builder, tag string, in []string) {
	if len(in) == 0 {
		return
	}
	output.WriteString("\n<" + tag + ">\n")
	if len(in) > 10 {
		output.WriteString(strings.Join(in[:10], "\n"))
		fmt.Fprintf(output, "\n... and %d more diagnostics", len(in)-10)
	} else {
		output.WriteString(strings.Join(in, "\n"))
	}
	output.WriteString("\n</" + tag + ">\n")
}

func sortDiagnostics(in []string) []string {
	sort.Slice(in, func(i, j int) bool {
		iIsError := strings.HasPrefix(in[i], "Error")
		jIsError := strings.HasPrefix(in[j], "Error")
		if iIsError != jIsError {
			return iIsError // Errors come first
		}
		return in[i] < in[j] // Then alphabetically
	})
	return in
}

func formatDiagnostic(pth string, diagnostic protocol.Diagnostic, source string) string {
	severity := "Info"
	switch diagnostic.Severity {
	case protocol.SeverityError:
		severity = "Error"
	case protocol.SeverityWarning:
		severity = "Warn"
	case protocol.SeverityHint:
		severity = "Hint"
	}

	location := fmt.Sprintf("%s:%d:%d", pth, diagnostic.Range.Start.Line+1, diagnostic.Range.Start.Character+1)

	sourceInfo := ""
	if diagnostic.Source != "" {
		sourceInfo = diagnostic.Source
	} else if source != "" {
		sourceInfo = source
	}

	codeInfo := ""
	if diagnostic.Code != nil {
		codeInfo = fmt.Sprintf("[%v]", diagnostic.Code)
	}

	tagsInfo := ""
	if len(diagnostic.Tags) > 0 {
		tags := []string{}
		for _, tag := range diagnostic.Tags {
			switch tag {
			case protocol.Unnecessary:
				tags = append(tags, "unnecessary")
			case protocol.Deprecated:
				tags = append(tags, "deprecated")
			}
		}
		if len(tags) > 0 {
			tagsInfo = fmt.Sprintf(" (%s)", strings.Join(tags, ", "))
		}
	}

	return fmt.Sprintf("%s: %s [%s]%s%s %s",
		severity,
		location,
		sourceInfo,
		codeInfo,
		tagsInfo,
		diagnostic.Message)
}

func countSeverity(diagnostics []string, severity string) int {
	count := 0
	for _, diag := range diagnostics {
		if strings.HasPrefix(diag, severity) {
			count++
		}
	}
	return count
}
