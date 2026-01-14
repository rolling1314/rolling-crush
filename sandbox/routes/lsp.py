"""
LSP 诊断路由 - 在容器内运行语言服务器/linter获取诊断信息
"""

import os
import json
from flask import Blueprint, request, jsonify, current_app

lsp_bp = Blueprint('lsp', __name__)


def get_sandbox_from_session(session_manager, session_id):
    """
    通过 session_id 获取 sandbox 实例
    """
    if not session_id:
        raise ValueError("session_id is required")
    
    sandbox = session_manager.get_or_create(session_id)
    return sandbox


def detect_language(file_path: str) -> str:
    """
    根据文件扩展名检测语言
    """
    ext = os.path.splitext(file_path)[1].lower()
    
    language_map = {
        '.py': 'python',
        '.go': 'go',
        '.js': 'javascript',
        '.jsx': 'javascript',
        '.ts': 'typescript',
        '.tsx': 'typescript',
        '.java': 'java',
        '.rs': 'rust',
        '.rb': 'ruby',
        '.php': 'php',
        '.c': 'c',
        '.cpp': 'cpp',
        '.cxx': 'cpp',
        '.cc': 'cpp',
        '.h': 'c',
        '.hpp': 'cpp',
        '.cs': 'csharp',
        '.sh': 'shell',
        '.bash': 'shell',
    }
    
    return language_map.get(ext, 'unknown')


def get_python_diagnostics(sandbox, file_path: str) -> list:
    """
    使用 Python linter 获取诊断信息
    支持 pylint, pyflakes, flake8
    """
    diagnostics = []
    
    # 尝试使用 pyflakes (轻量级)
    pyflakes_script = f'''
import sys
import json

try:
    from pyflakes import api
    from pyflakes import reporter as mod_reporter
    
    class JSONReporter:
        def __init__(self):
            self.errors = []
        
        def unexpectedError(self, filename, msg):
            self.errors.append({{
                "line": 1,
                "character": 0,
                "severity": 1,  # Error
                "message": str(msg),
                "source": "pyflakes"
            }})
        
        def syntaxError(self, filename, msg, lineno, offset, text):
            self.errors.append({{
                "line": lineno or 1,
                "character": offset or 0,
                "severity": 1,  # Error
                "message": str(msg),
                "source": "pyflakes"
            }})
        
        def flake(self, message):
            self.errors.append({{
                "line": message.lineno,
                "character": getattr(message, 'col', 0),
                "severity": 2 if 'undefined' in str(message).lower() else 2,  # Warning
                "message": str(message),
                "source": "pyflakes"
            }})
    
    reporter = JSONReporter()
    
    with open("{file_path}", "r") as f:
        code = f.read()
    
    api.check(code, "{file_path}", reporter)
    print(json.dumps(reporter.errors))
    
except ImportError:
    # pyflakes 未安装，尝试基本语法检查
    import ast
    errors = []
    try:
        with open("{file_path}", "r") as f:
            code = f.read()
        ast.parse(code)
    except SyntaxError as e:
        errors.append({{
            "line": e.lineno or 1,
            "character": e.offset or 0,
            "severity": 1,
            "message": str(e.msg),
            "source": "python-syntax"
        }})
    print(json.dumps(errors))
except Exception as e:
    print(json.dumps([{{"line": 1, "character": 0, "severity": 1, "message": str(e), "source": "linter-error"}}]))
'''
    
    result = sandbox.run_code(pyflakes_script, language='python')
    
    if result['exit_code'] == 0 and result['stdout'].strip():
        try:
            errors = json.loads(result['stdout'].strip())
            for err in errors:
                diagnostics.append({
                    "range": {
                        "start": {"line": err.get("line", 1) - 1, "character": err.get("character", 0)},
                        "end": {"line": err.get("line", 1) - 1, "character": err.get("character", 0) + 1}
                    },
                    "severity": err.get("severity", 1),
                    "source": err.get("source", "python"),
                    "message": err.get("message", "Unknown error")
                })
        except json.JSONDecodeError:
            pass
    
    return diagnostics


def get_go_diagnostics(sandbox, file_path: str) -> list:
    """
    使用 Go 工具链获取诊断信息
    """
    diagnostics = []
    
    # 获取文件所在目录
    file_dir = os.path.dirname(file_path) or '/sandbox'
    
    # 使用 go vet 和 go build 检查
    check_script = f'''
cd "{file_dir}" 2>/dev/null || cd /sandbox

# 首先尝试 go vet
go vet "{file_path}" 2>&1 | while read line; do
    echo "$line"
done

# 然后尝试语法检查
go build -o /dev/null "{file_path}" 2>&1 | while read line; do
    echo "$line"
done
'''
    
    result = sandbox.run_code(check_script, language='bash')
    
    # 解析 Go 错误输出格式: file.go:line:col: message
    import re
    error_pattern = re.compile(r'^(.+?):(\d+):(\d+)?:?\s*(.+)$')
    
    for line in (result['stdout'] + result['stderr']).split('\n'):
        line = line.strip()
        if not line:
            continue
        
        match = error_pattern.match(line)
        if match:
            _, line_num, col, message = match.groups()
            line_num = int(line_num) if line_num else 1
            col = int(col) if col else 0
            
            # 判断严重程度
            severity = 1  # Error by default
            if 'warning' in message.lower():
                severity = 2
            
            diagnostics.append({
                "range": {
                    "start": {"line": line_num - 1, "character": col},
                    "end": {"line": line_num - 1, "character": col + 1}
                },
                "severity": severity,
                "source": "go",
                "message": message
            })
    
    return diagnostics


def get_javascript_diagnostics(sandbox, file_path: str) -> list:
    """
    使用 ESLint 或基本语法检查获取诊断信息
    """
    diagnostics = []
    
    # 尝试使用 Node.js 进行语法检查
    check_script = f'''
const fs = require('fs');
const path = require('path');

try {{
    const code = fs.readFileSync("{file_path}", "utf8");
    const errors = [];
    
    // 基本语法检查
    try {{
        new Function(code);
    }} catch (e) {{
        // 解析错误消息
        const match = e.message.match(/Unexpected token.*at position (\\d+)/);
        const lineMatch = e.stack?.match(/:(\d+):(\d+)/);
        
        errors.push({{
            line: lineMatch ? parseInt(lineMatch[1]) : 1,
            character: lineMatch ? parseInt(lineMatch[2]) : 0,
            severity: 1,
            message: e.message,
            source: "javascript"
        }});
    }}
    
    console.log(JSON.stringify(errors));
}} catch (e) {{
    console.log(JSON.stringify([{{
        line: 1,
        character: 0,
        severity: 1,
        message: e.message,
        source: "javascript"
    }}]));
}}
'''
    
    # 写入临时脚本并执行
    temp_script = '/tmp/js_check.js'
    sandbox.write_file(temp_script, check_script)
    result = sandbox.run_code(f'node {temp_script}', language='bash')
    
    if result['stdout'].strip():
        try:
            errors = json.loads(result['stdout'].strip())
            for err in errors:
                diagnostics.append({
                    "range": {
                        "start": {"line": err.get("line", 1) - 1, "character": err.get("character", 0)},
                        "end": {"line": err.get("line", 1) - 1, "character": err.get("character", 0) + 1}
                    },
                    "severity": err.get("severity", 1),
                    "source": err.get("source", "javascript"),
                    "message": err.get("message", "Unknown error")
                })
        except json.JSONDecodeError:
            pass
    
    return diagnostics


def get_typescript_diagnostics(sandbox, file_path: str) -> list:
    """
    使用 TypeScript 编译器获取诊断信息
    """
    diagnostics = []
    
    # 使用 tsc 进行类型检查
    file_dir = os.path.dirname(file_path) or '/sandbox'
    
    check_script = f'''
cd "{file_dir}" 2>/dev/null || cd /sandbox

# 尝试使用 npx tsc
if command -v npx &> /dev/null; then
    npx --yes typescript --noEmit --pretty false "{file_path}" 2>&1
elif command -v tsc &> /dev/null; then
    tsc --noEmit --pretty false "{file_path}" 2>&1
else
    echo "TypeScript compiler not found"
fi
'''
    
    result = sandbox.run_code(check_script, language='bash')
    
    # 解析 TypeScript 错误输出格式: file.ts(line,col): error TSxxxx: message
    import re
    error_pattern = re.compile(r'^(.+?)\((\d+),(\d+)\):\s*(error|warning)\s+TS\d+:\s*(.+)$')
    
    for line in (result['stdout'] + result['stderr']).split('\n'):
        line = line.strip()
        if not line:
            continue
        
        match = error_pattern.match(line)
        if match:
            _, line_num, col, severity_str, message = match.groups()
            line_num = int(line_num) if line_num else 1
            col = int(col) if col else 0
            
            severity = 1 if severity_str == 'error' else 2
            
            diagnostics.append({
                "range": {
                    "start": {"line": line_num - 1, "character": col - 1},
                    "end": {"line": line_num - 1, "character": col}
                },
                "severity": severity,
                "source": "typescript",
                "message": message
            })
    
    return diagnostics


def get_diagnostics_for_file(sandbox, file_path: str) -> list:
    """
    根据文件类型获取诊断信息
    """
    language = detect_language(file_path)
    
    print(f"🔍 [LSP] 检测语言: {language} (文件: {file_path})", flush=True)
    
    if language == 'python':
        return get_python_diagnostics(sandbox, file_path)
    elif language == 'go':
        return get_go_diagnostics(sandbox, file_path)
    elif language == 'javascript':
        return get_javascript_diagnostics(sandbox, file_path)
    elif language == 'typescript':
        return get_typescript_diagnostics(sandbox, file_path)
    else:
        # 对于不支持的语言，返回空诊断
        print(f"⚠️ [LSP] 不支持的语言: {language}", flush=True)
        return []


@lsp_bp.route('/lsp/diagnostics', methods=['POST'])
def get_lsp_diagnostics():
    """
    获取 LSP 诊断信息
    
    请求体:
    {
        "session_id": "xxx",
        "file_path": "/sandbox/main.py"  // 可选，为空则返回项目级诊断
    }
    
    响应:
    {
        "status": "ok",
        "file_diagnostics": [
            {
                "file_path": "/sandbox/main.py",
                "diagnostics": [
                    {
                        "range": {"start": {"line": 0, "character": 0}, "end": {"line": 0, "character": 1}},
                        "severity": 1,
                        "source": "python",
                        "message": "Syntax error"
                    }
                ]
            }
        ],
        "project_diagnostics": []
    }
    """
    try:
        session_manager = current_app.config.get('session_manager')
        data = request.json
        session_id = data.get('session_id')
        file_path = data.get('file_path', '')
        
        print(f"\n📨 [/lsp/diagnostics] 收到请求", flush=True)
        print(f"   会话ID: {session_id}", flush=True)
        print(f"   文件路径: {file_path}", flush=True)
        
        # 获取 sandbox 实例
        sandbox = get_sandbox_from_session(session_manager, session_id)
        
        file_diagnostics = []
        project_diagnostics = []
        
        if file_path:
            # 获取指定文件的诊断
            diagnostics = get_diagnostics_for_file(sandbox, file_path)
            if diagnostics:
                file_diagnostics.append({
                    "file_path": file_path,
                    "diagnostics": diagnostics
                })
            
            print(f"✅ [/lsp/diagnostics] 文件诊断完成, 发现 {len(diagnostics)} 个问题", flush=True)
        else:
            # 项目级诊断 - 遍历常见源文件
            print(f"🔍 [/lsp/diagnostics] 执行项目级诊断...", flush=True)
            
            # 查找项目中的源文件
            find_script = '''
find /sandbox -type f \\( -name "*.py" -o -name "*.go" -o -name "*.js" -o -name "*.ts" -o -name "*.jsx" -o -name "*.tsx" \\) 2>/dev/null | head -50
'''
            result = sandbox.run_code(find_script, language='bash')
            
            if result['exit_code'] == 0 and result['stdout'].strip():
                files = [f.strip() for f in result['stdout'].strip().split('\n') if f.strip()]
                
                for src_file in files:
                    diagnostics = get_diagnostics_for_file(sandbox, src_file)
                    if diagnostics:
                        project_diagnostics.append({
                            "file_path": src_file,
                            "diagnostics": diagnostics
                        })
            
            print(f"✅ [/lsp/diagnostics] 项目诊断完成, 检查了 {len(files) if result['exit_code'] == 0 else 0} 个文件", flush=True)
        
        return jsonify({
            "status": "ok",
            "file_diagnostics": file_diagnostics,
            "project_diagnostics": project_diagnostics
        })
        
    except ValueError as e:
        print(f"❌ [/lsp/diagnostics] 参数错误: {str(e)}", flush=True)
        return jsonify({"status": "error", "error": str(e)}), 400
    except Exception as e:
        print(f"❌ [/lsp/diagnostics] 异常: {str(e)}", flush=True)
        import traceback
        traceback.print_exc()
        return jsonify({"status": "error", "error": str(e)}), 500
