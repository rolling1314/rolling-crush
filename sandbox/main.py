"""
自建 Docker 沙箱 - 在阿里云主机上运行
无需第三方服务，完全自托管

使用前需要在服务器上安装 Docker:
    curl -fsSL https://get.docker.com | sh
    systemctl start docker
    systemctl enable docker
"""

from __future__ import annotations

import docker
import tempfile
import os
import tarfile
import io
import json
import time
from typing import Optional, Dict
from flask import Flask, request, jsonify
from threading import Lock


class SessionManager:
    """会话容器管理器 - 维护会话ID到沙箱容器的映射"""
    
    def __init__(self):
        self.sessions: Dict[str, Sandbox] = {}
        self.lock = Lock()
    
    def get_or_create(self, session_id: str, **sandbox_kwargs) -> Sandbox:
        """获取或创建会话对应的沙箱容器"""
        with self.lock:
            if session_id not in self.sessions:
                print(f"🆕 创建新沙箱容器 (会话: {session_id})", flush=True)
                sandbox = Sandbox(**sandbox_kwargs)
                sandbox.start()
                self.sessions[session_id] = sandbox
            else:
                # 容器已存在，检查是否还在运行
                sandbox = self.sessions[session_id]
                if sandbox.container:
                    try:
                        sandbox.container.reload()
                        if sandbox.container.status != 'running':
                            print(f"⚠️ 容器已停止，重新启动 (会话: {session_id})", flush=True)
                            sandbox.start()
                    except Exception as e:
                        print(f"⚠️ 容器检查失败，重新创建 (会话: {session_id}): {e}", flush=True)
                        sandbox = Sandbox(**sandbox_kwargs)
                        sandbox.start()
                        self.sessions[session_id] = sandbox
            return self.sessions[session_id]
    
    def get(self, session_id: str) -> Optional[Sandbox]:
        """获取会话对应的沙箱容器"""
        with self.lock:
            return self.sessions.get(session_id)
    
    def remove(self, session_id: str):
        """移除并销毁会话对应的沙箱容器"""
        with self.lock:
            if session_id in self.sessions:
                sandbox = self.sessions[session_id]
                sandbox.stop()
                del self.sessions[session_id]
                print(f"🗑️ 移除沙箱容器 (会话: {session_id})")
    
    def list_sessions(self):
        """列出所有活跃会话"""
        with self.lock:
            return list(self.sessions.keys())
    
    def cleanup_all(self):
        """清理所有沙箱容器"""
        with self.lock:
            for session_id in list(self.sessions.keys()):
                sandbox = self.sessions[session_id]
                sandbox.stop()
            self.sessions.clear()


class Sandbox:
    """基于 Docker 的代码沙箱"""
    
    @staticmethod
    def _detect_docker_socket() -> str:
        """自动检测 Docker socket 路径"""
        # 常见的 Docker socket 路径
        socket_paths = [
            "/var/run/docker.sock",  # 默认 Linux / Docker Desktop
            os.path.expanduser("~/.orbstack/run/docker.sock"),  # OrbStack
            os.path.expanduser("~/.docker/run/docker.sock"),  # Docker Desktop (新版)
            os.path.expanduser("~/.colima/docker.sock"),  # Colima
            os.path.expanduser("~/.colima/default/docker.sock"),  # Colima default
        ]
        
        for path in socket_paths:
            if os.path.exists(path):
                print(f"🔍 检测到 Docker socket: {path}")
                return f"unix://{path}"
        
        return None
    
    def __init__(
        self,
        image: str = "python:3.11-slim",
        timeout: int = 30,
        memory_limit: str = "256m",
        cpu_limit: float = 0.5,
        docker_host: str = None,
        destroy_delay: int = 0
    ):
        """
        初始化沙箱
        
        Args:
            image: Docker 镜像名称
            timeout: 代码执行超时时间(秒)
            memory_limit: 内存限制 (如 "256m", "1g")
            cpu_limit: CPU 限制 (0.5 = 50% 单核)
            docker_host: Docker socket 路径 (自动检测)
            destroy_delay: 销毁前等待时间(秒)，默认0立即销毁
        """
        # 自动检测 Docker socket 路径
        if docker_host is None:
            docker_host = self._detect_docker_socket()
        
        if docker_host:
            self.client = docker.DockerClient(base_url=docker_host)
        else:
            self.client = docker.from_env()
        
        self.image = image
        self.timeout = timeout
        self.memory_limit = memory_limit
        self.cpu_limit = cpu_limit
        self.destroy_delay = destroy_delay
        self.container = None
        
    def __enter__(self):
        """启动沙箱容器"""
        self.start()
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """销毁沙箱容器"""
        self.stop()
        
    def start(self):
        """启动容器"""
        print(f"🚀 正在启动沙箱 (镜像: {self.image})...")
        
        # 拉取镜像（如果不存在）
        try:
            self.client.images.get(self.image)
        except docker.errors.ImageNotFound:
            print(f"📥 正在拉取镜像 {self.image}...")
            self.client.images.pull(self.image)
        
        # 创建并启动容器
        self.container = self.client.containers.run(
            self.image,
            command="sleep infinity",  # 保持容器运行
            detach=True,
            mem_limit=self.memory_limit,
            nano_cpus=int(self.cpu_limit * 1e9),
            network_disabled=True,  # 禁用网络（安全）
            read_only=False,
            working_dir="/sandbox",
            # 安全限制
            security_opt=["no-new-privileges"],
            cap_drop=["ALL"],  # 移除所有特权
        )
        
        # 创建工作目录
        self.container.exec_run("mkdir -p /sandbox")
        print(f"✅ 沙箱已启动 (容器ID: {self.container.short_id})")
        
    def stop(self):
        """停止并删除容器"""
        if self.container:
            try:
                if self.destroy_delay > 0:
                    import time
                    print(f"⏳ 等待 {self.destroy_delay} 秒后销毁沙箱...")
                    print(f"   容器ID: {self.container.short_id}")
                    print(f"   你可以使用 'docker exec -it {self.container.short_id} bash' 进入容器")
                    time.sleep(self.destroy_delay)
                self.container.stop(timeout=1)
                self.container.remove(force=True)
                print("🔴 沙箱已销毁")
            except Exception as e:
                print(f"⚠️ 停止容器时出错: {e}")
            finally:
                self.container = None
            
    def run_code(self, code: str, language: str = "python") -> dict:
        """
        在沙箱中执行代码
        
        Args:
            code: 要执行的代码
            language: 编程语言 (目前支持 python, bash)
            
        Returns:
            {"stdout": str, "stderr": str, "exit_code": int}
        """
        if not self.container:
            raise RuntimeError("沙箱未启动，请先调用 start() 或使用 with 语句")
        
        # 根据语言选择执行命令
        if language == "python":
            cmd = ["python", "-c", code]
        elif language == "bash":
            cmd = ["bash", "-c", code]
        else:
            raise ValueError(f"不支持的语言: {language}")
        
        try:
            result = self.container.exec_run(
                cmd,
                workdir="/sandbox",
                demux=True,  # 分离 stdout 和 stderr
            )
            
            stdout = result.output[0].decode("utf-8") if result.output[0] else ""
            stderr = result.output[1].decode("utf-8") if result.output[1] else ""
            
            return {
                "stdout": stdout,
                "stderr": stderr,
                "exit_code": result.exit_code
            }
            
        except Exception as e:
            return {
                "stdout": "",
                "stderr": str(e),
                "exit_code": -1
            }
    
    def write_file(self, path: str, content: str):
        """
        在沙箱中写入文件
        
        Args:
            path: 文件路径 (相对于 /sandbox)
            content: 文件内容
        """
        if not self.container:
            raise RuntimeError("沙箱未启动")
            
        # 确保路径在 /sandbox 下
        full_path = f"/sandbox/{path.lstrip('/')}"
        
        # 创建 tar 归档并上传
        data = content.encode("utf-8")
        tarstream = io.BytesIO()
        
        with tarfile.open(fileobj=tarstream, mode="w") as tar:
            tarinfo = tarfile.TarInfo(name=os.path.basename(full_path))
            tarinfo.size = len(data)
            tar.addfile(tarinfo, io.BytesIO(data))
        
        tarstream.seek(0)
        self.container.put_archive(os.path.dirname(full_path), tarstream)
        
    def read_file(self, path: str) -> str:
        """
        读取沙箱中的文件
        
        Args:
            path: 文件路径 (相对于 /sandbox)
            
        Returns:
            文件内容
        """
        if not self.container:
            raise RuntimeError("沙箱未启动")
            
        full_path = f"/sandbox/{path.lstrip('/')}"
        result = self.container.exec_run(["cat", full_path])
        
        if result.exit_code != 0:
            raise FileNotFoundError(f"文件不存在: {path}")
            
        return result.output.decode("utf-8")
    
    def list_files(self, path: str = "/sandbox") -> list:
        """
        列出沙箱中的文件
        
        Args:
            path: 目录路径
            
        Returns:
            文件名列表
        """
        if not self.container:
            raise RuntimeError("沙箱未启动")
            
        result = self.container.exec_run(["ls", "-1", path])
        if result.exit_code != 0:
            return []
            
        files = result.output.decode("utf-8").strip().split("\n")
        return [f for f in files if f]


def main():
    """使用示例"""
    
    # 测试模式：完成后立即销毁容器
    with Sandbox(memory_limit="256m", cpu_limit=0.5, destroy_delay=0) as sandbox:
        
        # 1. 执行 Python 代码
        print("\n📌 执行系统信息查询:")
        result = sandbox.run_code("""
import platform
import sys
print(f"系统: {platform.system()} {platform.release()}")
print(f"Python: {sys.version}")
""")
        print(result["stdout"])
        
        # 2. 数学计算
        print("📌 执行数学计算:")
        result = sandbox.run_code("""
result = sum(range(1, 101))
print(f"1到100的和: {result}")

import math
print(f"圆周率: {math.pi:.10f}")
""")
        print(result["stdout"])
        
        # 3. 文件操作
        print("📌 文件操作:")
        sandbox.write_file("hello.txt", "你好，这是沙箱中的文件！\nHello Sandbox!")
        content = sandbox.read_file("hello.txt")
        print(f"文件内容:\n{content}")
        
        # 4. 列出文件
        files = sandbox.list_files()
        print(f"文件列表: {files}")
        
        # 5. 执行 Bash 命令
        print("\n📌 执行 Bash 命令:")
        result = sandbox.run_code("echo '当前目录:' && pwd && ls -la", language="bash")
        print(result["stdout"])
        
        # 6. 错误处理演示
        print("📌 错误处理:")
        result = sandbox.run_code("print(1/0)")
        if result["stderr"]:
            print(f"捕获错误: {result['stderr'][:100]}...")


def interactive_mode():
    """交互式沙箱模式"""
    
    with Sandbox() as sandbox:
        print("\n🎮 交互式沙箱 (输入 'exit' 退出, 'bash:' 前缀执行bash命令)")
        print("-" * 50)
        
        while True:
            try:
                code = input("\n>>> ")
                
                if code.lower() == "exit":
                    break
                if not code.strip():
                    continue
                
                # 判断是否是 bash 命令
                if code.startswith("bash:"):
                    result = sandbox.run_code(code[5:].strip(), language="bash")
                else:
                    result = sandbox.run_code(code)
                
                if result["stdout"]:
                    print(result["stdout"], end="")
                if result["stderr"]:
                    print(f"❌ {result['stderr']}", end="")
                    
            except KeyboardInterrupt:
                print("\n中断...")
                break


# Flask应用和API
app = Flask(__name__)
session_manager = SessionManager()


@app.route('/health', methods=['GET'])
def health():
    """健康检查"""
    return jsonify({"status": "ok", "active_sessions": len(session_manager.sessions)})


@app.route('/sessions', methods=['GET'])
def list_sessions():
    """列出所有活跃会话"""
    sessions = session_manager.list_sessions()
    print(f"\n📨 [GET /sessions] 查询活跃会话")
    print(f"   活跃会话数: {len(sessions)}")
    return jsonify({
        "sessions": sessions,
        "count": len(sessions)
    })

@app.route('/sessions/cleanup', methods=['POST'])
def cleanup_all_sessions():
    """清理所有会话和容器"""
    print(f"\n📨 [POST /sessions/cleanup] 收到清理请求")
    count = len(session_manager.sessions)
    session_manager.cleanup_all()
    print(f"✅ [POST /sessions/cleanup] 已清理 {count} 个会话")
    return jsonify({
        "status": "ok",
        "message": f"Cleaned up {count} sessions"
    })


@app.route('/session/<session_id>', methods=['DELETE'])
def delete_session(session_id):
    """删除会话和对应的容器"""
    print(f"\n📨 [DELETE /session] 收到删除请求")
    print(f"   会话ID: {session_id}")
    
    session = session_manager.get(session_id)
    if session:
        session_manager.remove(session_id)
        print(f"✅ [DELETE /session] 会话已删除")
        return jsonify({"status": "ok", "message": f"Session {session_id} removed"})
    else:
        print(f"⚠️ [DELETE /session] 会话不存在")
        return jsonify({"status": "ok", "message": f"Session {session_id} not found"})


@app.route('/execute', methods=['POST'])
def execute_code():
    """执行代码 - 对应 bash 工具"""
    try:
        data = request.json
        session_id = data.get('session_id')
        command = data.get('command')
        language = data.get('language', 'bash')
        working_dir = data.get('working_dir', '/sandbox')
        
        print(f"\n📨 [/execute] 收到请求", flush=True)
        print(f"   会话ID: {session_id}", flush=True)
        print(f"   命令: {command}", flush=True)
        print(f"   语言: {language}", flush=True)
        
        if not session_id or not command:
            print(f"❌ [/execute] 参数缺失")
            return jsonify({"error": "session_id and command are required"}), 400
        
        sandbox = session_manager.get_or_create(session_id)
        result = sandbox.run_code(command, language)
        
        print(f"✅ [/execute] 执行完成, 退出码: {result['exit_code']}", flush=True)
        if result['stdout']:
            print(f"   标准输出: {result['stdout'][:100]}...", flush=True)
        if result['stderr']:
            print(f"   标准错误: {result['stderr'][:100]}...", flush=True)
        
        return jsonify({
            "status": "ok",
            "stdout": result["stdout"],
            "stderr": result["stderr"],
            "exit_code": result["exit_code"]
        })
    except Exception as e:
        print(f"❌ [/execute] 异常: {str(e)}")
        return jsonify({"error": str(e)}), 500


@app.route('/file/read', methods=['POST'])
def read_file():
    """读取文件 - 对应 view 工具"""
    try:
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


@app.route('/file/write', methods=['POST'])
def write_file():
    """写入文件 - 对应 write 和 edit 工具"""
    try:
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


@app.route('/file/list', methods=['POST'])
def list_files():
    """列出文件 - 对应 ls 工具"""
    try:
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


@app.route('/file/grep', methods=['POST'])
def grep_file():
    """搜索文件内容 - 对应 grep 工具"""
    try:
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


@app.route('/file/glob', methods=['POST'])
def glob_search():
    """文件名模式匹配 - 对应 glob 工具"""
    try:
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


@app.route('/file/edit', methods=['POST'])
def edit_file():
    """编辑文件内容 - 对应 edit 工具（搜索替换）"""
    try:
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


@app.route('/diagnostic', methods=['POST'])
def get_diagnostics():
    """获取诊断信息 - 对应 diagnostics 工具"""
    try:
        data = request.json
        session_id = data.get('session_id')
        file_path = data.get('file_path')
        
        if not session_id:
            return jsonify({"error": "session_id is required"}), 400
        
        sandbox = session_manager.get_or_create(session_id)
        
        # 目前返回空的诊断信息，后续可以集成 LSP
        return jsonify({
            "status": "ok",
            "diagnostics": []
        })
    except Exception as e:
        return jsonify({"error": str(e)}), 500


def run_server(host='0.0.0.0', port=8888, auto_cleanup=False):
    """运行Flask服务器
    
    Args:
        host: 监听地址
        port: 监听端口
        auto_cleanup: 服务器停止时是否自动清理容器（默认False，保持容器运行）
    """
    print(f"🚀 沙箱服务启动在 http://{host}:{port}", flush=True)
    print(f"📝 API端点:", flush=True)
    print(f"   - POST /execute         执行命令", flush=True)
    print(f"   - POST /file/read       读取文件", flush=True)
    print(f"   - POST /file/write      写入文件", flush=True)
    print(f"   - POST /file/list       列出文件", flush=True)
    print(f"   - POST /file/grep       搜索内容", flush=True)
    print(f"   - POST /file/glob       搜索文件名", flush=True)
    print(f"   - POST /file/edit       编辑文件", flush=True)
    print(f"   - GET  /health          健康检查", flush=True)
    print(f"   - GET  /sessions        列出会话", flush=True)
    print(f"   - DELETE /session/<id>  删除会话", flush=True)
    print(f"\n⚙️ 容器策略: {'服务停止时自动清理' if auto_cleanup else '保持运行（手动清理）'}", flush=True)
    print(f"💡 提示: 容器会保持运行以提高性能，使用 DELETE /session/<id> 手动清理", flush=True)
    
    try:
        app.run(host=host, port=port, debug=False, threaded=True)
    finally:
        if auto_cleanup:
            print("\n🛑 正在清理所有沙箱容器...")
            session_manager.cleanup_all()
        else:
            print("\n⏸️ 服务停止，容器保持运行")
            print(f"   当前活跃会话: {len(session_manager.sessions)}")
            print(f"   💡 容器将继续运行，重启服务后可继续使用")


if __name__ == "__main__":
    import sys
    
    if len(sys.argv) > 1 and sys.argv[1] == "server":
        # 运行服务器模式
        print("=" * 60, flush=True)
        print("🌐 启动沙箱服务器模式", flush=True)
        print("=" * 60, flush=True)
        run_server()
    else:
        # 运行测试模式
        print("=" * 60, flush=True)
        print("🧪 运行测试模式（不是服务器）", flush=True)
        print("💡 如需启动服务器，请运行: python main.py server", flush=True)
        print("=" * 60, flush=True)
        main()
        
        # 交互模式
        # interactive_mode()

