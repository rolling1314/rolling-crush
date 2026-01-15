package tools

import (
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/fantasy"
	"github.com/rolling1314/rolling-crush/internal/pkg/fsext"
	"github.com/rolling1314/rolling-crush/infra/sandbox"
)

const GlobToolName = "glob"

//go:embed glob.md
var globDescription []byte

type GlobParams struct {
	Pattern string `json:"pattern" description:"The glob pattern to match files against"`
	Path    string `json:"path,omitempty" description:"The directory to search in. Defaults to the current working directory."`
}

type GlobResponseMetadata struct {
	NumberOfFiles int  `json:"number_of_files"`
	Truncated     bool `json:"truncated"`
}

func NewGlobTool(workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		GlobToolName,
		string(globDescription),
		func(ctx context.Context, params GlobParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Pattern == "" {
				return fantasy.NewTextErrorResponse("pattern is required"), nil
			}

			contextWorkingDir := GetWorkingDirFromContext(ctx)
			effectiveWorkingDir := cmp.Or(contextWorkingDir, workingDir)
			searchPath := params.Path
			if searchPath == "" {
				searchPath = effectiveWorkingDir
			}
			
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for finding files")
			}

		// ============== 路由到沙箱服务 ==============
		sandboxClient := sandbox.GetDefaultClient()

		resp, err := sandboxClient.Glob(ctx, sandbox.GlobRequest{
			SessionID: sessionID,
			Pattern:   params.Pattern,
			Path:      searchPath,
		})
			
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error finding files from sandbox: %w", err)
			}

			output := strings.TrimSpace(resp.Stdout)
			if output == "" {
				output = "No files found"
			}
			
			// 统计文件数
			fileCount := 0
			if output != "No files found" {
				fileCount = strings.Count(output, "\n") + 1
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output),
				GlobResponseMetadata{
					NumberOfFiles: fileCount,
					Truncated:     false,
				},
			), nil
			
			// ============== 原本地文件查找代码（已注释） ==============
			/*
			files, truncated, err := globFiles(ctx, params.Pattern, searchPath, 100)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error finding files: %w", err)
			}

			var output string
			if len(files) == 0 {
				output = "No files found"
			} else {
				normalizeFilePaths(files)
				output = strings.Join(files, "\n")
				if truncated {
					output += "\n\n(Results are truncated. Consider using a more specific path or pattern.)"
				}
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output),
				GlobResponseMetadata{
					NumberOfFiles: len(files),
					Truncated:     truncated,
				},
			), nil
			*/
		})
}

func globFiles(ctx context.Context, pattern, searchPath string, limit int) ([]string, bool, error) {
	cmdRg := getRgCmd(ctx, pattern)
	if cmdRg != nil {
		cmdRg.Dir = searchPath
		matches, err := runRipgrep(cmdRg, searchPath, limit)
		if err == nil {
			return matches, len(matches) >= limit && limit > 0, nil
		}
		slog.Warn("Ripgrep execution failed, falling back to doublestar", "error", err)
	}

	return fsext.GlobWithDoubleStar(pattern, searchPath, limit)
}

func runRipgrep(cmd *exec.Cmd, searchRoot string, limit int) ([]string, error) {
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("ripgrep: %w\n%s", err, out)
	}

	var matches []string
	for p := range bytes.SplitSeq(out, []byte{0}) {
		if len(p) == 0 {
			continue
		}
		absPath := string(p)
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(searchRoot, absPath)
		}
		if fsext.SkipHidden(absPath) {
			continue
		}
		matches = append(matches, absPath)
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return len(matches[i]) < len(matches[j])
	})

	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

func normalizeFilePaths(paths []string) {
	for i, p := range paths {
		paths[i] = filepath.ToSlash(p)
	}
}
