"""
基于 Docker 的代码沙箱
"""

import os
import io
import tarfile
import docker


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
        self.workdir = "/sandbox"  # 默认工作目录
        
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
            
            # 保存工作目录
            self.workdir = workdir
            
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
            language: 编程语言 (目前支持 python, bash, sh)
            
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
        elif language == "sh":
            cmd = ["sh", "-c", code]
        else:
            raise ValueError(f"不支持的语言: {language}")
        
        try:
            result = self.container.exec_run(
                cmd,
                workdir=self.workdir,
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

        # 标准化路径：如果是绝对路径就直接使用，否则添加工作目录前缀
        if path.startswith('/'):
            full_path = path
        else:
            full_path = f"{self.workdir}/{path}"
        
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

        # 标准化路径：如果是绝对路径就直接使用，否则添加工作目录前缀
        if path.startswith('/'):
            full_path = path
        else:
            full_path = f"{self.workdir}/{path}"
        result = self.container.exec_run(["cat", full_path])
        
        if result.exit_code != 0:
            raise FileNotFoundError(f"文件不存在: {path}")
            
        return result.output.decode("utf-8")
    
    def list_files(self, path: str = None) -> list:
        """
        列出沙箱中的文件
        
        Args:
            path: 目录路径，默认为工作目录
            
        Returns:
            文件名列表
        """
        if not self.container:
            raise RuntimeError("沙箱未启动")
        
        # 如果没有指定路径，使用工作目录
        if path is None:
            path = self.workdir
            
        result = self.container.exec_run(["ls", "-1", path])
        if result.exit_code != 0:
            return []
            
        files = result.output.decode("utf-8").strip().split("\n")
        return [f for f in files if f]
