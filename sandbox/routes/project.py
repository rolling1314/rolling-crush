"""
项目管理路由
"""

import os
import time
import socket
import docker
import traceback
import subprocess
from flask import Blueprint, request, jsonify, current_app
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
            image_name = "my-app1"
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


@project_bp.route('/projects/delete', methods=['POST'])
def delete_project():
    """删除项目容器 - 停止并删除Docker容器"""
    try:
        data = request.json
        container_id = data.get('container_id')
        
        print(f"\n📨 [POST /projects/delete] 收到删除项目请求", flush=True)
        print(f"   容器ID: {container_id}", flush=True)
        
        if not container_id:
            print(f"❌ [POST /projects/delete] 容器ID不能为空")
            return jsonify({"error": "container_id is required"}), 400
        
        # 连接Docker
        docker_socket = Sandbox._detect_docker_socket()
        if docker_socket:
            client = docker.DockerClient(base_url=docker_socket)
            print(f"   使用 Docker socket: {docker_socket}", flush=True)
        else:
            client = docker.from_env()
            print(f"   使用默认 Docker 连接", flush=True)
        
        try:
            # 查找容器（支持短ID和完整ID）
            container = client.containers.get(container_id)
            container_name = container.name
            print(f"   找到容器: {container_name} (状态: {container.status})", flush=True)
            
            # 停止容器（如果正在运行）
            if container.status == 'running':
                print(f"   正在停止容器...", flush=True)
                container.stop(timeout=10)
                print(f"   容器已停止", flush=True)
            
            # 删除容器
            print(f"   正在删除容器...", flush=True)
            container.remove(force=True)
            
            print(f"✅ [POST /projects/delete] 容器删除成功: {container_name}", flush=True)
            
            return jsonify({
                "status": "ok",
                "message": f"Container {container_name} deleted successfully"
            })
            
        except docker.errors.NotFound:
            print(f"⚠️ [POST /projects/delete] 容器不存在: {container_id}", flush=True)
            # 容器不存在，视为删除成功
            return jsonify({
                "status": "ok",
                "message": f"Container {container_id} not found, considered deleted"
            })
            
    except Exception as e:
        print(f"❌ [POST /projects/delete] 异常: {str(e)}", flush=True)
        traceback.print_exc()
        return jsonify({"error": str(e)}), 500


@project_bp.route('/projects/configure-domain', methods=['POST'])
def configure_domain():
    """配置项目域名 - 添加nginx配置和更新vite配置"""
    try:
        data = request.json
        container_id = data.get('container_id')
        subdomain = data.get('subdomain')  # 三级域名前缀，如 "abc1234567"
        frontend_port = data.get('frontend_port')  # 主机端口
        domain = data.get('domain', 'rollingcoding.com')  # 基础域名
        
        print(f"\n📨 [POST /projects/configure-domain] 收到配置域名请求", flush=True)
        print(f"   容器ID: {container_id}", flush=True)
        print(f"   三级域名: {subdomain}.{domain}", flush=True)
        print(f"   前端端口: {frontend_port}", flush=True)
        
        if not container_id:
            return jsonify({"error": "container_id is required"}), 400
        if not subdomain:
            return jsonify({"error": "subdomain is required"}), 400
        if not frontend_port:
            return jsonify({"error": "frontend_port is required"}), 400
        
        full_subdomain = f"{subdomain}.{domain}"
        nginx_config_path = f"/etc/nginx/sites-available/{domain}.conf"
        
        # 1. 生成并添加 nginx server block
        nginx_server_block = f'''
# {full_subdomain} - 项目子域名反向代理
server {{
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name {full_subdomain};
    location / {{
        proxy_pass http://127.0.0.1:{frontend_port};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }}
}}
'''
        
        print(f"   正在添加 nginx 配置...", flush=True)
        try:
            # 追加 nginx 配置到文件
            with open(nginx_config_path, 'a') as f:
                f.write(nginx_server_block)
            print(f"   ✅ nginx 配置已添加", flush=True)
        except Exception as e:
            print(f"   ❌ 添加 nginx 配置失败: {e}", flush=True)
            return jsonify({"error": f"Failed to add nginx config: {str(e)}"}), 500
        
        # 2. 更新容器内的 vite.config.ts
        vite_config_content = f'''import {{ defineConfig }} from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({{
  plugins: [react()],
  server: {{
    host: '0.0.0.0',
    port: {frontend_port},
    allowedHosts: [
      '{full_subdomain}',
      '.{domain}',
    ],
  }},
}})
'''
        
        print(f"   正在更新容器内 vite.config.ts...", flush=True)
        try:
            # 连接 Docker
            docker_socket = Sandbox._detect_docker_socket()
            if docker_socket:
                client = docker.DockerClient(base_url=docker_socket)
            else:
                client = docker.from_env()
            
            # 获取容器
            container = client.containers.get(container_id)
            
            # 写入 vite.config.ts 到容器
            # 使用 docker exec 来写入文件
            exec_result = container.exec_run(
                cmd=['sh', '-c', f'cat > /workspace/frontend/vite.config.ts << \'EOF\'\n{vite_config_content}\nEOF'],
                workdir='/workspace'
            )
            
            if exec_result.exit_code != 0:
                print(f"   ⚠️ 更新 vite.config.ts 可能失败: {exec_result.output.decode()}", flush=True)
            else:
                print(f"   ✅ vite.config.ts 已更新", flush=True)
                
        except docker.errors.NotFound:
            print(f"   ⚠️ 容器不存在，跳过 vite 配置: {container_id}", flush=True)
        except Exception as e:
            print(f"   ⚠️ 更新 vite.config.ts 失败: {e}", flush=True)
            # 不返回错误，因为 nginx 配置已经成功
        
        # 3. 重新加载 nginx
        print(f"   正在重新加载 nginx...", flush=True)
        try:
            result = subprocess.run(['nginx', '-s', 'reload'], capture_output=True, text=True)
            if result.returncode != 0:
                print(f"   ⚠️ nginx 重载失败: {result.stderr}", flush=True)
            else:
                print(f"   ✅ nginx 已重新加载", flush=True)
        except Exception as e:
            print(f"   ⚠️ nginx 重载失败: {e}", flush=True)
        
        print(f"✅ [POST /projects/configure-domain] 域名配置完成: {full_subdomain}", flush=True)
        
        return jsonify({
            "status": "ok",
            "subdomain": full_subdomain,
            "message": f"Domain {full_subdomain} configured successfully"
        })
        
    except Exception as e:
        print(f"❌ [POST /projects/configure-domain] 异常: {str(e)}", flush=True)
        traceback.print_exc()
        return jsonify({"error": str(e)}), 500


@project_bp.route('/projects/startup', methods=['POST'])
def startup_project():
    """触发项目容器启动动作，在容器内执行可配置的 bash 命令"""
    try:
        data = request.json or {}
        project_id = data.get('project_id')

        if not project_id:
            return jsonify({"error": "project_id is required"}), 400

        config = current_app.config.get('config')
        startup_cfg = config.get('sandbox.startup', {}) if config else {}

        command = (data.get('command') or startup_cfg.get('command') or '').strip()
        language = (data.get('language') or startup_cfg.get('language') or 'bash').strip()
        working_dir = (data.get('working_dir') or startup_cfg.get('working_dir') or '/workspace').strip()

        if not command:
            return jsonify({"error": "startup command is empty, please configure sandbox.startup.command in config.yaml"}), 400

        session_manager = current_app.config.get('session_manager')
        if not session_manager:
            return jsonify({"error": "session_manager not initialized"}), 500

        print(f"\n📨 [POST /projects/startup] 收到项目启动请求", flush=True)
        print(f"   项目ID: {project_id}", flush=True)
        print(f"   命令: {command}", flush=True)
        print(f"   工作目录: {working_dir}", flush=True)

        sandbox = session_manager.get_or_create_by_project(project_id)

        old_workdir = sandbox.workdir
        sandbox.workdir = working_dir
        try:
            result = sandbox.run_code(command, language)
        finally:
            sandbox.workdir = old_workdir

        return jsonify({
            "status": "ok",
            "stdout": result.get("stdout", ""),
            "stderr": result.get("stderr", ""),
            "exit_code": result.get("exit_code", 0),
            "command": command
        })

    except ValueError as e:
        return jsonify({"error": str(e)}), 400
    except docker.errors.NotFound as e:
        return jsonify({"error": f"容器不存在: {str(e)}"}), 404
    except RuntimeError as e:
        return jsonify({"error": str(e)}), 503
    except Exception as e:
        print(f"❌ [POST /projects/startup] 异常: {str(e)}", flush=True)
        traceback.print_exc()
        return jsonify({"error": str(e)}), 500
