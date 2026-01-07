"""
自建 Docker 沙箱 - 在阿里云主机上运行
无需第三方服务，完全自托管

使用前需要在服务器上安装 Docker:
    curl -fsSL https://get.docker.com | sh
    systemctl start docker
    systemctl enable docker
"""

import docker
import tempfile
import os
import tarfile
import io
from typing import Optional


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
            if self.destroy_delay > 0:
                import time
                print(f"⏳ 等待 {self.destroy_delay} 秒后销毁沙箱...")
                print(f"   容器ID: {self.container.short_id}")
                print(f"   你可以使用 'docker exec -it {self.container.short_id} bash' 进入容器")
                time.sleep(self.destroy_delay)
            self.container.stop(timeout=1)
            self.container.remove(force=True)
            print("🔴 沙箱已销毁")
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
    
    # destroy_delay=180 表示完成后等待3分钟再销毁
    with Sandbox(memory_limit="256m", cpu_limit=0.5, destroy_delay=180) as sandbox:
        
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


if __name__ == "__main__":
    main()
    
    # 交互模式
    # interactive_mode()

