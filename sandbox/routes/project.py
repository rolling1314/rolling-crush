"""
项目管理路由
"""

import time
import socket
import docker
import traceback
from flask import Blueprint, request, jsonify
from sandbox import Sandbox

project_bp = Blueprint('project', __name__)


@project_bp.route('/projects/create', methods=['POST'])
def create_project():
    """创建项目容器 - 启动Docker容器并分配端口"""
    try:
        data = request.json
        project_name = data.get('project_name')
        backend_language = data.get('backend_language')  # '', 'go', 'java', 'python'
        need_database = data.get('need_database', False)
        
        print(f"\n📨 [POST /projects/create] 收到创建项目请求", flush=True)
        print(f"   项目名称: {project_name}", flush=True)
        print(f"   后端语言: {backend_language or 'None'}", flush=True)
        print(f"   需要数据库: {need_database}", flush=True)
        
        if not project_name:
            print(f"❌ [POST /projects/create] 项目名称不能为空")
            return jsonify({"error": "project_name is required"}), 400
        
        # 根据语言选择镜像
        if backend_language == 'go':
            image_name = "go-vite"
        elif backend_language == 'java':
            image_name = "java-vite"
        elif backend_language == 'python':
            image_name = "python-vite"
        else:
            # 纯前端项目
            image_name = "vite-dev"
        
        # 查找可用端口
        def find_available_port(start_port=8000, end_port=9000):
            """查找可用的主机端口"""
            for port in range(start_port, end_port):
                try:
                    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
                        s.bind(('', port))
                        return port
                except OSError:
                    continue
            raise RuntimeError(f"No available ports in range {start_port}-{end_port}")
        
        # 分配端口
        frontend_host_port = find_available_port(8000, 8500)
        backend_host_port = find_available_port(8500, 9000) if backend_language else None
        
        print(f"   分配的前端端口: {frontend_host_port} (容器端口: 5173)", flush=True)
        if backend_host_port:
            print(f"   分配的后端端口: {backend_host_port} (容器端口: 8888)", flush=True)
        
        # 构建容器名称
        container_name = f"{project_name.lower().replace(' ', '-')}-{int(time.time())}"
        
        # 启动容器 - 使用自动检测的 Docker socket
        docker_socket = Sandbox._detect_docker_socket()
        if docker_socket:
            client = docker.DockerClient(base_url=docker_socket)
            print(f"   使用 Docker socket: {docker_socket}", flush=True)
        else:
            client = docker.from_env()
            print(f"   使用默认 Docker 连接", flush=True)
        
        # 检查镜像是否存在
        try:
            client.images.get(image_name)
            print(f"   使用镜像: {image_name}", flush=True)
        except docker.errors.ImageNotFound:
            print(f"❌ [POST /projects/create] 镜像 {image_name} 不存在", flush=True)
            return jsonify({"error": f"Docker image '{image_name}' not found. Please build it first."}), 400
        
        # 构建端口映射
        port_bindings = {
            '5173/tcp': frontend_host_port
        }
        if backend_host_port:
            port_bindings['8888/tcp'] = backend_host_port
        
        # 启动容器
        print(f"   正在启动容器: {container_name}...", flush=True)
        container = client.containers.run(
            image_name,
            name=container_name,
            detach=True,
            ports=port_bindings,
            environment={
                'PROJECT_NAME': project_name,
                'BACKEND_LANGUAGE': backend_language or '',
                'NEED_DATABASE': str(need_database).lower()
            },
            restart_policy={"Name": "unless-stopped"}
        )
        
        # 等待容器启动
        time.sleep(2)
        container.reload()
        
        # 使用容器ID（短ID，12位）作为标识符
        container_id = container.id
        container_short_id = container.short_id  # 这是12位的短ID
        
        print(f"✅ [POST /projects/create] 容器创建成功", flush=True)
        print(f"   容器ID (短): {container_short_id}", flush=True)
        print(f"   容器ID (完整): {container_id}", flush=True)
        print(f"   容器名称: {container_name}", flush=True)
        print(f"   状态: {container.status}", flush=True)
        
        return jsonify({
            "status": "ok",
            "container_id": container_short_id,  # 返回12位短ID
            "container_name": container_name,
            "frontend_port": frontend_host_port,
            "backend_port": backend_host_port,
            "image": image_name,
            "workdir": "/workspace",  # 工作目录
            "message": f"Project container created successfully"
        })
        
    except Exception as e:
        print(f"❌ [POST /projects/create] 异常: {str(e)}", flush=True)
        traceback.print_exc()
        return jsonify({"error": str(e)}), 500
