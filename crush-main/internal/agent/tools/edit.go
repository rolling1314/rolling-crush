package tools

import (
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/pkg/csync"
	"github.com/charmbracelet/crush/internal/pkg/diff"
	"github.com/charmbracelet/crush/internal/pkg/filepathext"
	"github.com/charmbracelet/crush/internal/pkg/fsext"
	"github.com/charmbracelet/crush/domain/history"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/domain/permission"
	"github.com/charmbracelet/crush/sandbox"
)

type EditParams struct {
	FilePath   string `json:"file_path" description:"The absolute path to the file to modify"`
	OldString  string `json:"old_string" description:"The text to replace"`
	NewString  string `json:"new_string" description:"The text to replace it with"`
	ReplaceAll bool   `json:"replace_all,omitempty" description:"Replace all occurrences of old_string (default false)"`
}

type EditPermissionsParams struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

type EditResponseMetadata struct {
	Additions  int    `json:"additions"`
	Removals   int    `json:"removals"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

const EditToolName = "edit"

//go:embed edit.md
var editDescription []byte

type editContext struct {
	ctx         context.Context
	permissions permission.Service
	files       history.Service
	workingDir  string
}

func NewEditTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, files history.Service, workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		EditToolName,
		string(editDescription),
		func(ctx context.Context, params EditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.FilePath == "" {
				return fantasy.NewTextErrorResponse("file_path is required"), nil
			}

			contextWorkingDir := GetWorkingDirFromContext(ctx)
			effectiveWorkingDir := cmp.Or(contextWorkingDir, workingDir)
			params.FilePath = filepathext.SmartJoin(effectiveWorkingDir, params.FilePath)

			var response fantasy.ToolResponse
			var err error

			editCtx := editContext{ctx, permissions, files, effectiveWorkingDir}

			if params.OldString == "" {
				response, err = createNewFile(editCtx, params.FilePath, params.NewString, call)
				if err != nil {
					return response, err
				}
			}

			if params.NewString == "" {
				response, err = deleteContent(editCtx, params.FilePath, params.OldString, params.ReplaceAll, call)
				if err != nil {
					return response, err
				}
			}

			response, err = replaceContent(editCtx, params.FilePath, params.OldString, params.NewString, params.ReplaceAll, call)
			if err != nil {
				return response, err
			}
			if response.IsError {
				// Return early if there was an error during content replacement
				// This prevents unnecessary LSP diagnostics processing
				return response, nil
			}

			notifyLSPs(ctx, lspClients, params.FilePath)

			text := fmt.Sprintf("<result>\n%s\n</result>\n", response.Content)
			text += getDiagnostics(params.FilePath, lspClients)
			response.Content = text
			return response, nil
		})
}

func createNewFile(edit editContext, filePath, content string, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	sessionID := GetSessionFromContext(edit.ctx)
	if sessionID == "" {
		return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for creating a new file")
	}

	// ============== 路由到沙箱服务 ==============
	sandboxClient := sandbox.GetDefaultClient()

	// 检查文件是否已存在
	_, err := sandboxClient.ReadFile(edit.ctx, sandbox.FileReadRequest{
		SessionID: sessionID,
		FilePath:  filePath,
	})
	if err == nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("file already exists: %s", filePath)), nil
	}

	p := edit.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        fsext.PathOrPrefix(filePath, edit.workingDir),
			ToolCallID:  call.ID,
			ToolName:    EditToolName,
			Action:      "write",
			Description: fmt.Sprintf("Create file %s", filePath),
			Params: EditPermissionsParams{
				FilePath:   filePath,
				OldContent: "",
				NewContent: content,
			},
		},
	)
	if !p {
		return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
	}

	// 写入新文件
	_, err = sandboxClient.WriteFile(edit.ctx, sandbox.FileWriteRequest{
		SessionID: sessionID,
		FilePath:  filePath,
		Content:   content,
	})
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("failed to write file to sandbox: %w", err)
	}

	_, additions, removals := diff.GenerateDiff(
		"",
		content,
		strings.TrimPrefix(filePath, edit.workingDir),
	)

	// File can't be in the history so we create a new file history
	_, err = edit.files.Create(edit.ctx, sessionID, filePath, "")
	if err != nil {
		// Log error but don't fail the operation
		slog.Error("Error creating file history", "error", err)
	}

	// Add the new content to the file history
	_, err = edit.files.CreateVersion(edit.ctx, sessionID, filePath, content)
	if err != nil {
		// Log error but don't fail the operation
		slog.Error("Error creating file history version", "error", err)
	}

	recordFileWrite(filePath)
	recordFileRead(filePath)

	return fantasy.WithResponseMetadata(
		fantasy.NewTextResponse("File created: "+filePath),
		EditResponseMetadata{
			OldContent: "",
			NewContent: content,
			Additions:  additions,
			Removals:   removals,
		},
	), nil

	// ============== 原本地文件创建代码（已注释） ==============
	/*
		fileInfo, err := os.Stat(filePath)
		if err == nil {
			if fileInfo.IsDir() {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("path is a directory, not a file: %s", filePath)), nil
			}
			return fantasy.NewTextErrorResponse(fmt.Sprintf("file already exists: %s", filePath)), nil
		} else if !os.IsNotExist(err) {
			return fantasy.ToolResponse{}, fmt.Errorf("failed to access file: %w", err)
		}

		dir := filepath.Dir(filePath)
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return fantasy.ToolResponse{}, fmt.Errorf("failed to create parent directories: %w", err)
		}

		p := edit.permissions.Request(
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        fsext.PathOrPrefix(filePath, edit.workingDir),
				ToolCallID:  call.ID,
				ToolName:    EditToolName,
				Action:      "write",
				Description: fmt.Sprintf("Create file %s", filePath),
				Params: EditPermissionsParams{
					FilePath:   filePath,
					OldContent: "",
					NewContent: content,
				},
			},
		)
		if !p {
			return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
		}

		err = os.WriteFile(filePath, []byte(content), 0o644)
		if err != nil {
			return fantasy.ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
		}
	*/
}

func deleteContent(edit editContext, filePath, oldString string, replaceAll bool, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	sessionID := GetSessionFromContext(edit.ctx)
	if sessionID == "" {
		return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for editing file")
	}

	// ============== 路由到沙箱服务 ==============
	sandboxClient := sandbox.GetDefaultClient()

	// 读取文件内容
	resp, err := sandboxClient.ReadFile(edit.ctx, sandbox.FileReadRequest{
		SessionID: sessionID,
		FilePath:  filePath,
	})
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("file not found: %s", filePath)), nil
	}

	oldContent, isCrlf := fsext.ToUnixLineEndings(resp.Content)

	var newContent string
	var deletionCount int

	if replaceAll {
		newContent = strings.ReplaceAll(oldContent, oldString, "")
		deletionCount = strings.Count(oldContent, oldString)
		if deletionCount == 0 {
			return fantasy.NewTextErrorResponse("old_string not found in file. Make sure it matches exactly, including whitespace and line breaks"), nil
		}
	} else {
		index := strings.Index(oldContent, oldString)
		if index == -1 {
			return fantasy.NewTextErrorResponse("old_string not found in file. Make sure it matches exactly, including whitespace and line breaks"), nil
		}

		lastIndex := strings.LastIndex(oldContent, oldString)
		if index != lastIndex {
			return fantasy.NewTextErrorResponse("old_string appears multiple times in the file. Please provide more context to ensure a unique match, or set replace_all to true"), nil
		}

		newContent = oldContent[:index] + oldContent[index+len(oldString):]
		deletionCount = 1
	}

	_, additions, removals := diff.GenerateDiff(
		oldContent,
		newContent,
		strings.TrimPrefix(filePath, edit.workingDir),
	)

	if isCrlf {
		newContent, _ = fsext.ToWindowsLineEndings(newContent)
	}

	p := edit.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        fsext.PathOrPrefix(filePath, edit.workingDir),
			ToolCallID:  call.ID,
			ToolName:    EditToolName,
			Action:      "edit",
			Description: fmt.Sprintf("Delete content in %s", filePath),
			Params: EditPermissionsParams{
				FilePath:   filePath,
				OldContent: oldContent,
				NewContent: newContent,
			},
		},
	)
	if !p {
		return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
	}

	// 写回文件
	_, err = sandboxClient.WriteFile(edit.ctx, sandbox.FileWriteRequest{
		SessionID: sessionID,
		FilePath:  filePath,
		Content:   newContent,
	})
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("failed to write file to sandbox: %w", err)
	}

	// Check if file exists in history
	file, err := edit.files.GetByPathAndSession(edit.ctx, filePath, sessionID)
	if err != nil {
		_, err = edit.files.Create(edit.ctx, sessionID, filePath, oldContent)
		if err != nil {
			// Log error but don't fail the operation
			slog.Error("Error creating file history", "error", err)
		}
	}
	if file.Content != oldContent {
		// User Manually changed the content store an intermediate version
		_, err = edit.files.CreateVersion(edit.ctx, sessionID, filePath, oldContent)
		if err != nil {
			slog.Error("Error creating file history version", "error", err)
		}
	}
	// Store the new version
	_, err = edit.files.CreateVersion(edit.ctx, sessionID, filePath, "")
	if err != nil {
		slog.Error("Error creating file history version", "error", err)
	}

	recordFileWrite(filePath)
	recordFileRead(filePath)

	return fantasy.WithResponseMetadata(
		fantasy.NewTextResponse("Content deleted from file: "+filePath),
		EditResponseMetadata{
			OldContent: oldContent,
			NewContent: newContent,
			Additions:  additions,
			Removals:   removals,
		},
	), nil
}

func replaceContent(edit editContext, filePath, oldString, newString string, replaceAll bool, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	sessionID := GetSessionFromContext(edit.ctx)
	if sessionID == "" {
		return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for editing file")
	}

	// ============== 路由到沙箱服务 ==============
	sandboxClient := sandbox.GetDefaultClient()

	// 读取文件内容
	resp, err := sandboxClient.ReadFile(edit.ctx, sandbox.FileReadRequest{
		SessionID: sessionID,
		FilePath:  filePath,
	})
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("file not found: %s", filePath)), nil
	}

	oldContent, isCrlf := fsext.ToUnixLineEndings(resp.Content)

	var newContent string
	var replacementCount int

	if replaceAll {
		newContent = strings.ReplaceAll(oldContent, oldString, newString)
		replacementCount = strings.Count(oldContent, oldString)
		if replacementCount == 0 {
			return fantasy.NewTextErrorResponse("old_string not found in file. Make sure it matches exactly, including whitespace and line breaks"), nil
		}
	} else {
		index := strings.Index(oldContent, oldString)
		if index == -1 {
			return fantasy.NewTextErrorResponse("old_string not found in file. Make sure it matches exactly, including whitespace and line breaks"), nil
		}

		lastIndex := strings.LastIndex(oldContent, oldString)
		if index != lastIndex {
			return fantasy.NewTextErrorResponse("old_string appears multiple times in the file. Please provide more context to ensure a unique match, or set replace_all to true"), nil
		}

		newContent = oldContent[:index] + newString + oldContent[index+len(oldString):]
		replacementCount = 1
	}

	if oldContent == newContent {
		return fantasy.NewTextErrorResponse("new content is the same as old content. No changes made."), nil
	}

	_, additions, removals := diff.GenerateDiff(
		oldContent,
		newContent,
		strings.TrimPrefix(filePath, edit.workingDir),
	)

	if isCrlf {
		newContent, _ = fsext.ToWindowsLineEndings(newContent)
	}

	p := edit.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        fsext.PathOrPrefix(filePath, edit.workingDir),
			ToolCallID:  call.ID,
			ToolName:    EditToolName,
			Action:      "edit",
			Description: fmt.Sprintf("Replace content in %s", filePath),
			Params: EditPermissionsParams{
				FilePath:   filePath,
				OldContent: oldContent,
				NewContent: newContent,
			},
		},
	)
	if !p {
		return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
	}

	// 写回文件
	_, err = sandboxClient.WriteFile(edit.ctx, sandbox.FileWriteRequest{
		SessionID: sessionID,
		FilePath:  filePath,
		Content:   newContent,
	})
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("failed to write file to sandbox: %w", err)
	}

	// Check if file exists in history
	file, err := edit.files.GetByPathAndSession(edit.ctx, filePath, sessionID)
	if err != nil {
		_, err = edit.files.Create(edit.ctx, sessionID, filePath, oldContent)
		if err != nil {
			// Log error but don't fail the operation
			slog.Error("Error creating file history", "error", err)
		}
	}
	if file.Content != oldContent {
		// User Manually changed the content store an intermediate version
		_, err = edit.files.CreateVersion(edit.ctx, sessionID, filePath, oldContent)
		if err != nil {
			slog.Debug("Error creating file history version", "error", err)
		}
	}
	// Store the new version
	_, err = edit.files.CreateVersion(edit.ctx, sessionID, filePath, newContent)
	if err != nil {
		slog.Error("Error creating file history version", "error", err)
	}

	recordFileWrite(filePath)
	recordFileRead(filePath)

	return fantasy.WithResponseMetadata(
		fantasy.NewTextResponse("Content replaced in file: "+filePath),
		EditResponseMetadata{
			OldContent: oldContent,
			NewContent: newContent,
			Additions:  additions,
			Removals:   removals,
		}), nil
}
