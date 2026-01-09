"""
自建 Docker 沙箱 - 在阿里云主机上运行
无需第三方服务，完全自托管

使用前需要在服务器上安装 Docker:
    curl -fsSL https://get.docker.com | sh
    systemctl start docker
    systemctl enable docker

安装 PostgreSQL 客户端:
    pip install psycopg2-binary
"""

from __future__ import annotations

import docker
import tempfile
import os
import tarfile
import io
import json
import time
import psycopg2
from psycopg2.extras import RealDictCursor
from typing import Optional, Dict, Tuple
from flask import Flask, request, jsonify
from threading import Lock


class DatabaseManager:
    """PostgreSQL 数据库管理器 - 查询会话和项目信息"""
    
    def __init__(self):
        """初始化数据库连接，使用与 Go 代码相同的环境变量"""
        self.host = os.getenv("POSTGRES_HOST", "localhost")
        self.port = os.getenv("POSTGRES_PORT", "5432")
        self.user = os.getenv("POSTGRES_USER", "crush")
        self.password = os.getenv("POSTGRES_PASSWORD", "123456")
        self.database = os.getenv("POSTGRES_DB", "crush")
        self.sslmode = os.getenv("POSTGRES_SSLMODE", "disable")
        self.conn = None
        self._connect()
    
    def _connect(self):
        """建立数据库连接"""
        try:
            self.conn = psycopg2.connect(
                host=self.host,
                port=self.port,
                user=self.user,
                password=self.password,
                database=self.database,
                sslmode=self.sslmode
            )
            print(f"✅ 数据库连接成功: {self.user}@{self.host}:{self.port}/{self.database}")
        except Exception as e:
            print(f"⚠️ 数据库连接失败: {e}")
            print(f"   将以独立模式运行（不连接数据库）")
            self.conn = None
    
    def get_project_by_session(self, session_id: str) -> Optional[Dict]:
        """根据会话ID查询项目信息
        
        返回:
            {
                'id': 项目ID,
                'name': 项目名称,
                'container_name': 容器名称,
                'workdir_path': 工作目录路径,
                'host': 主机地址,
                'port': 端口,
                'workspace_path': 工作空间路径
            }
        """
        if not self.conn:
            return None
        
        try:
            with self.conn.cursor(cursor_factory=RealDictCursor) as cursor:
                # 联合查询 sessions 和 projects 表
                cursor.execute("""
                    SELECT 
                        p.id,
                        p.name,
                        p.container_name,
                        p.workdir_path,
                        p.host,
                        p.port,
                        p.workspace_path
                    FROM sessions s
                    JOIN projects p ON s.project_id = p.id
                    WHERE s.id = %s
                    LIMIT 1
                """, (session_id,))
                
                result = cursor.fetchone()
                if result:
                    return dict(result)
                return None
        except Exception as e:
            print(f"⚠️ 查询数据库失败: {e}")
            # 尝试重新连接
            try:
                self.conn.close()
            except:
                pass
            self._connect()
            return None
    
    def close(self):
        """关闭数据库连接"""
        if self.conn:
            try:
                self.conn.close()
                print("📊 数据库连接已关闭")
            except:
                pass


class SessionManager:
    """会话容器管理器 - 维护会话ID到沙箱容器的映射"""
    
    def __init__(self, db_manager: Optional[DatabaseManager] = None):
        self.sessions: Dict[str, Sandbox] = {}
        self.lock = Lock()
        self.db = db_manager
    
    def get_or_create(self, session_id: str, **sandbox_kwargs) -> Sandbox:
        """获取会话对应的容器（仅连接现有容器，不创建新容器）
        
        工作流程：
        1. 从数据库查询会话对应的项目信息
        2. 如果项目有 container_name，连接到该容器
        3. 如果没有容器信息，抛出异常
        """
        with self.lock:
            if session_id not in self.sessions:
                # 必须从数据库查询项目信息
                if not self.db:
                    raise RuntimeError("数据库未连接，无法查询容器信息")
                
                project_info = self.db.get_project_by_session(session_id)
                
                if not project_info:
                    raise ValueError(f"会话 {session_id} 不存在或未关联项目")
                
                if not project_info.get('container_name'):
                    raise ValueError(
                        f"项目 '{project_info.get('name', 'Unknown')}' 尚未配置容器。"
                        f"请先在项目设置中配置 container_name"
                    )
                
                # 连接到现有容器
                container_name = project_info['container_name']
                workdir = project_info.get('workdir_path') or '/sandbox'
                
                print(f"🔗 连接到项目容器 (会话: {session_id})", flush=True)
                print(f"   项目: {project_info.get('name', 'Unknown')}", flush=True)
                print(f"   容器: {container_name}", flush=True)
                print(f"   工作目录: {workdir}", flush=True)
                
                sandbox = Sandbox(**sandbox_kwargs)
                sandbox.attach_to_existing(container_name, workdir)
                self.sessions[session_id] = sandbox
            else:
                # 容器已在缓存中，检查状态
                sandbox = self.sessions[session_id]
                if sandbox.container:
                    try:
                        sandbox.container.reload()
                        if sandbox.container.status != 'running':
                            print(f"⚠️ 容器已停止，正在重启 (会话: {session_id})", flush=True)
                            sandbox.container.start()
                            sandbox.container.reload()
                    except docker.errors.NotFound:
                        # 容器被删除了，重新查询数据库
                        print(f"⚠️ 容器不存在，重新连接 (会话: {session_id})", flush=True)
                        del self.sessions[session_id]
                        return self.get_or_create(session_id, **sandbox_kwargs)
                    except Exception as e:
                        print(f"⚠️ 容器检查失败: {e}", flush=True)
                        # 重新连接
                        del self.sessions[session_id]
                        return self.get_or_create(session_id, **sandbox_kwargs)
            
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
    
    def attach_to_existing(self, container_name: str, workdir: str = "/sandbox"):
        """连接到现有的容器（容器必须存在）
        
        Args:
            container_name: 容器名称或ID
            workdir: 工作目录路径
            
        Raises:
            docker.errors.NotFound: 容器不存在
            RuntimeError: 连接失败
        """
        try:
            # 尝试通过名称获取容器
            self.container = self.client.containers.get(container_name)
            
            # 检查容器状态
            self.container.reload()
            if self.container.status != 'running':
                print(f"⚠️ 容器 {container_name} 未运行，正在启动...", flush=True)
                self.container.start()
                # 等待容器启动
                import time
                time.sleep(1)
                self.container.reload()
            
            # 确保工作目录存在
            result = self.container.exec_run(f"mkdir -p {workdir}")
            if result.exit_code != 0:
                print(f"⚠️ 创建工作目录失败: {result.output.decode()}", flush=True)
            
            print(f"✅ 已连接到容器: {container_name}", flush=True)
            print(f"   状态: {self.container.status}", flush=True)
            print(f"   工作目录: {workdir}", flush=True)
            
        except docker.errors.NotFound:
            raise docker.errors.NotFound(
                f"容器 '{container_name}' 不存在。请确保容器正在运行，或检查数据库中的 container_name 配置。"
            )
        except Exception as e:
            raise RuntimeError(f"连接容器 '{container_name}' 失败: {e}")
        
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
            path: 文件路径（绝对路径或相对路径）
            content: 文件内容
        """
        if not self.container:
            raise RuntimeError("沙箱未启动")

        # 标准化路径：如果是绝对路径就直接使用，否则添加 /sandbox 前缀
        if path.startswith('/'):
            full_path = path
        else:
            full_path = f"/sandbox/{path}"
        
        # 自动创建目录结构（类似 Go 的 os.MkdirAll）
        dir_path = os.path.dirname(full_path)
        if dir_path:
            result = self.container.exec_run(["mkdir", "-p", dir_path])
            if result.exit_code != 0:
                raise RuntimeError(f"创建目录失败: {result.output.decode()}")
        
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
            path: 文件路径（绝对路径或相对路径）

        Returns:
            文件内容
        """
        if not self.container:
            raise RuntimeError("沙箱未启动")

        # 标准化路径：如果是绝对路径就直接使用，否则添加 /sandbox 前缀
        if path.startswith('/'):
            full_path = path
        else:
            full_path = f"/sandbox/{path}"
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


# ============================================================
# Flask 后端服务 API
# ============================================================


app = Flask(__name__)

# 全局变量 - 延迟初始化
db_manager = None
session_manager = None

def init_managers():
    """初始化数据库和会话管理器（仅在服务器模式下调用）"""
    global db_manager, session_manager
    db_manager = DatabaseManager()
    session_manager = SessionManager(db_manager=db_manager)


@app.route('/health', methods=['GET'])
def health():
    """健康检查"""
    active_sessions = len(session_manager.sessions) if session_manager else 0
    return jsonify({"status": "ok", "active_sessions": active_sessions})


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
    except ValueError as e:
        # 业务逻辑错误（会话不存在、容器未配置等）
        print(f"❌ [/execute] 业务错误: {str(e)}", flush=True)
        return jsonify({"error": str(e)}), 400
    except docker.errors.NotFound as e:
        # 容器不存在
        print(f"❌ [/execute] 容器不存在: {str(e)}", flush=True)
        return jsonify({"error": f"容器不存在: {str(e)}"}), 404
    except RuntimeError as e:
        # 运行时错误（数据库未连接等）
        print(f"❌ [/execute] 运行时错误: {str(e)}", flush=True)
        return jsonify({"error": str(e)}), 503
    except Exception as e:
        # 未知错误
        print(f"❌ [/execute] 未知异常: {str(e)}", flush=True)
        import traceback
        traceback.print_exc()
        return jsonify({"error": f"内部错误: {str(e)}"}), 500


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


@app.route('/file/tree', methods=['GET'])
def get_file_tree():
    """获取文件树 - 对应前端文件浏览器"""
    try:
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


def run_server(host='0.0.0.0', port=8888, auto_cleanup=False):
    """运行Flask服务器
    
    Args:
        host: 监听地址
        port: 监听端口
        auto_cleanup: 服务器停止时是否自动清理容器（默认False，保持容器运行）
    """
    # 初始化管理器
    init_managers()
    
    print(f"🚀 沙箱服务启动在 http://{host}:{port}", flush=True)
    
    # 打印数据库连接状态
    if db_manager and db_manager.conn:
        print(f"📊 数据库: 已连接 ({db_manager.user}@{db_manager.host}:{db_manager.port}/{db_manager.database})", flush=True)
        print(f"   智能模式: 自动查询项目容器信息", flush=True)
    else:
        print(f"📊 数据库: 未连接，运行在独立模式", flush=True)
    
    print(f"\n📝 API端点:", flush=True)
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
        
        # 关闭数据库连接
        if db_manager:
            db_manager.close()


if __name__ == "__main__":
    print("=" * 60, flush=True)
    print("🚀 启动沙箱服务", flush=True)
    print("=" * 60, flush=True)
    run_server()

