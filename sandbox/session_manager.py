"""
会话容器管理器 - 维护会话ID到沙箱容器的映射
"""

import docker
from threading import Lock
from typing import Optional, Dict
from sandbox import Sandbox
from database import DatabaseManager


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
