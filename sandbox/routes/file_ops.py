"""
文件操作路由
"""

import json
from flask import Blueprint, request, jsonify, current_app

file_ops_bp = Blueprint('file_ops', __name__)


@file_ops_bp.route('/file/read', methods=['POST'])
def read_file():
    """读取文件 - 对应 view 工具"""
    try:
        session_manager = current_app.config.get('session_manager')
        data = request.json
        session_id = data.get('session_id')
        file_path = data.get('file_path')
        
        print(f"\n📨 [/file/read] 收到请求", flush=True)
        print(f"   会话ID: {session_id}", flush=True)
        print(f"   文件路径: {file_path}", flush=True)
        
        if not session_id or not file_path:
            print(f"❌ [/file/read] 参数缺失")
            return jsonify({"error": "session_id and file_path are required"}), 400
        
        sandbox = session_manager.get_or_create(session_id)
        content = sandbox.read_file(file_path)
        
        print(f"✅ [/file/read] 读取成功, 内容长度: {len(content)} 字节")
        
        return jsonify({
            "status": "ok",
            "content": content
        })
    except Exception as e:
        print(f"❌ [/file/read] 异常: {str(e)}")
        return jsonify({"error": str(e)}), 500


@file_ops_bp.route('/file/write', methods=['POST'])
def write_file():
    """写入文件 - 对应 write 和 edit 工具"""
    try:
        session_manager = current_app.config.get('session_manager')
        data = request.json
        session_id = data.get('session_id')
        file_path = data.get('file_path')
        content = data.get('content', '')
        
        print(f"\n📨 [/file/write] 收到请求", flush=True)
        print(f"   会话ID: {session_id}", flush=True)
        print(f"   文件路径: {file_path}", flush=True)
        print(f"   内容长度: {len(content)} 字节", flush=True)
        
        if not session_id or not file_path:
            print(f"❌ [/file/write] 参数缺失")
            return jsonify({"error": "session_id and file_path are required"}), 400
        
        sandbox = session_manager.get_or_create(session_id)
        sandbox.write_file(file_path, content)
        
        print(f"✅ [/file/write] 写入成功")
        
        return jsonify({
            "status": "ok",
            "message": f"File {file_path} written successfully"
        })
    except Exception as e:
        print(f"❌ [/file/write] 异常: {str(e)}")
        return jsonify({"error": str(e)}), 500


@file_ops_bp.route('/file/list', methods=['POST'])
def list_files():
    """列出文件 - 对应 ls 工具"""
    try:
        session_manager = current_app.config.get('session_manager')
        data = request.json
        session_id = data.get('session_id')
        path = data.get('path', '/sandbox')
        
        print(f"\n📨 [/file/list] 收到请求")
        print(f"   会话ID: {session_id}")
        print(f"   路径: {path}")
        
        if not session_id:
            print(f"❌ [/file/list] 参数缺失")
            return jsonify({"error": "session_id is required"}), 400
        
        sandbox = session_manager.get_or_create(session_id)
        files = sandbox.list_files(path)
        
        print(f"✅ [/file/list] 列出成功, 文件数: {len(files)}")
        
        return jsonify({
            "status": "ok",
            "files": files
        })
    except Exception as e:
        print(f"❌ [/file/list] 异常: {str(e)}")
        return jsonify({"error": str(e)}), 500


@file_ops_bp.route('/file/grep', methods=['POST'])
def grep_file():
    """搜索文件内容 - 对应 grep 工具"""
    try:
        session_manager = current_app.config.get('session_manager')
        data = request.json
        session_id = data.get('session_id')
        pattern = data.get('pattern')
        path = data.get('path', '/sandbox')
        
        print(f"\n📨 [/file/grep] 收到请求")
        print(f"   会话ID: {session_id}")
        print(f"   搜索模式: {pattern}")
        print(f"   路径: {path}")
        
        if not session_id or not pattern:
            print(f"❌ [/file/grep] 参数缺失")
            return jsonify({"error": "session_id and pattern are required"}), 400
        
        sandbox = session_manager.get_or_create(session_id)
        # 使用 grep 命令搜索
        cmd = f"grep -r '{pattern}' {path}"
        result = sandbox.run_code(cmd, language='bash')
        
        print(f"✅ [/file/grep] 搜索完成, 退出码: {result['exit_code']}")
        
        return jsonify({
            "status": "ok",
            "stdout": result["stdout"],
            "stderr": result["stderr"],
            "exit_code": result["exit_code"]
        })
    except Exception as e:
        print(f"❌ [/file/grep] 异常: {str(e)}")
        return jsonify({"error": str(e)}), 500


@file_ops_bp.route('/file/glob', methods=['POST'])
def glob_search():
    """文件名模式匹配 - 对应 glob 工具"""
    try:
        session_manager = current_app.config.get('session_manager')
        data = request.json
        session_id = data.get('session_id')
        pattern = data.get('pattern')
        path = data.get('path', '/sandbox')
        
        print(f"\n📨 [/file/glob] 收到请求")
        print(f"   会话ID: {session_id}")
        print(f"   搜索模式: {pattern}")
        print(f"   路径: {path}")
        
        if not session_id or not pattern:
            print(f"❌ [/file/glob] 参数缺失")
            return jsonify({"error": "session_id and pattern are required"}), 400
        
        sandbox = session_manager.get_or_create(session_id)
        # 使用 find 命令搜索文件名
        cmd = f"find {path} -name '{pattern}'"
        result = sandbox.run_code(cmd, language='bash')
        
        print(f"✅ [/file/glob] 搜索完成, 退出码: {result['exit_code']}")
        
        return jsonify({
            "status": "ok",
            "stdout": result["stdout"],
            "stderr": result["stderr"],
            "exit_code": result["exit_code"]
        })
    except Exception as e:
        print(f"❌ [/file/glob] 异常: {str(e)}")
        return jsonify({"error": str(e)}), 500


@file_ops_bp.route('/file/edit', methods=['POST'])
def edit_file():
    """编辑文件内容 - 对应 edit 工具（搜索替换）"""
    try:
        session_manager = current_app.config.get('session_manager')
        data = request.json
        session_id = data.get('session_id')
        file_path = data.get('file_path')
        old_string = data.get('old_string')
        new_string = data.get('new_string')
        replace_all = data.get('replace_all', False)
        
        print(f"\n📨 [/file/edit] 收到请求")
        print(f"   会话ID: {session_id}")
        print(f"   文件路径: {file_path}")
        print(f"   替换全部: {replace_all}")
        
        if not session_id or not file_path:
            print(f"❌ [/file/edit] 参数缺失")
            return jsonify({"error": "session_id and file_path are required"}), 400
        
        sandbox = session_manager.get_or_create(session_id)
        
        # 读取文件
        try:
            content = sandbox.read_file(file_path)
        except:
            content = ""
        
        # 执行替换
        if old_string:
            if replace_all:
                new_content = content.replace(old_string, new_string)
            else:
                # 只替换第一次出现
                new_content = content.replace(old_string, new_string, 1)
        else:
            # 没有 old_string，直接写入 new_string
            new_content = new_string
        
        # 写回文件
        sandbox.write_file(file_path, new_content)
        
        print(f"✅ [/file/edit] 编辑成功")
        
        return jsonify({
            "status": "ok",
            "message": f"File {file_path} edited successfully"
        })
    except Exception as e:
        print(f"❌ [/file/edit] 异常: {str(e)}")
        return jsonify({"error": str(e)}), 500


@file_ops_bp.route('/file/tree', methods=['GET'])
def get_file_tree():
    """获取文件树 - 对应前端文件浏览器"""
    try:
        session_manager = current_app.config.get('session_manager')
        # 从 query 参数获取
        session_id = request.args.get('session_id')
        target_path = request.args.get('path', '.')
        
        print(f"\n📨 [GET /file/tree] 收到请求", flush=True)
        print(f"   会话ID: {session_id}", flush=True)
        print(f"   目标路径: {target_path}", flush=True)
        
        if not session_id:
            print(f"❌ [GET /file/tree] 参数缺失")
            return jsonify({"error": "session_id is required"}), 400
        
        sandbox = session_manager.get_or_create(session_id)
        
        # 打印实际处理的容器路径
        if sandbox.container:
            print(f"   容器名称: {sandbox.container.name}", flush=True)
            print(f"   容器ID: {sandbox.container.short_id}", flush=True)
            print(f"   开始构建文件树...", flush=True)
        
        # 使用 Python 脚本在容器内生成文件树
        tree_script = f'''
import os
import json

def should_ignore(name):
    """检查文件是否应该被忽略"""
    ignore_patterns = [
        ".git", ".DS_Store", "node_modules", ".idea", ".vscode",
        "__pycache__", ".pytest_cache", ".pyc", ".pyo", ".env", ".env.local"
    ]
    return name in ignore_patterns or name.startswith('.')

def build_tree(path, root_path, counter):
    """递归构建文件树"""
    try:
        stat_info = os.stat(path)
    except Exception as e:
        return None
    
    # 计算相对路径
    rel_path = os.path.relpath(path, root_path)
    if rel_path == '.':
        rel_path = ''
    
    counter[0] += 1
    node = {{
        "id": str(counter[0]),
        "name": os.path.basename(path) if path != root_path else os.path.basename(root_path),
        "path": "/" + rel_path.replace(os.sep, "/") if rel_path else "/"
    }}
    
    if os.path.isdir(path):
        node["type"] = "folder"
        node["children"] = []
        
        try:
            entries = os.listdir(path)
            for entry in sorted(entries):
                if should_ignore(entry):
                    continue
                
                child_path = os.path.join(path, entry)
                child_node = build_tree(child_path, root_path, counter)
                if child_node:
                    node["children"].append(child_node)
        except Exception as e:
            pass
    else:
        node["type"] = "file"
        # 如果文件小于 1MB，读取内容
        if stat_info.st_size < 1024 * 1024:
            try:
                with open(path, 'r', encoding='utf-8') as f:
                    node["content"] = f.read()
            except:
                # 无法读取的文件（二进制文件等）不包含内容
                pass
    
    return node

# 获取目标路径
target = "{target_path}"
if not target.startswith('/'):
    target = os.path.join('/sandbox', target)

# 确保路径存在
if not os.path.exists(target):
    print(json.dumps({{"error": "Path does not exist: " + target}}))
else:
    counter = [0]
    tree = build_tree(target, target, counter)
    print(json.dumps(tree, ensure_ascii=False))
'''
        
        # 执行脚本
        result = sandbox.run_code(tree_script, language='python')
        
        if result['exit_code'] != 0:
            print(f"❌ [GET /file/tree] 生成文件树失败: {result['stderr']}", flush=True)
            return jsonify({"error": f"Failed to generate file tree: {result['stderr']}"}), 500
        
        # 解析返回的 JSON
        try:
            tree_data = json.loads(result['stdout'])
            if 'error' in tree_data:
                print(f"❌ [GET /file/tree] 路径错误: {tree_data['error']}", flush=True)
                return jsonify({"error": tree_data['error']}), 404
            
            # 打印文件树统计信息
            node_count = tree_data.get('id', 0)
            print(f"✅ [GET /file/tree] 文件树生成成功", flush=True)
            print(f"   节点总数: {node_count}", flush=True)
            print(f"   根节点: {tree_data.get('name', 'unknown')}", flush=True)
            
            return jsonify({
                "status": "ok",
                "tree": tree_data
            })
        except json.JSONDecodeError as e:
            print(f"❌ [GET /file/tree] JSON 解析失败: {str(e)}", flush=True)
            print(f"   输出内容: {result['stdout'][:500]}", flush=True)
            return jsonify({"error": f"Failed to parse tree data: {str(e)}"}), 500
            
    except Exception as e:
        print(f"❌ [GET /file/tree] 异常: {str(e)}", flush=True)
        import traceback
        traceback.print_exc()
        return jsonify({"error": str(e)}), 500
