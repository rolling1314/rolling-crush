"""
PostgreSQL 数据库管理器 - 查询会话和项目信息
"""

import os
import psycopg2
from psycopg2.extras import RealDictCursor
from typing import Optional, Dict


class DatabaseManager:
    """PostgreSQL 数据库管理器 - 查询会话和项目信息"""
    
    def __init__(self):
        """初始化数据库连接，使用与 Go 代码相同的环境变量"""
        self.host = os.getenv("POSTGRES_HOST", "120.26.101.52")
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
                'external_ip': 外部IP地址,
                'frontend_port': 前端端口,
                'workspace_path': 工作空间路径,
                'db_host': 数据库主机,
                'db_port': 数据库端口,
                'db_user': 数据库用户,
                'db_password': 数据库密码,
                'db_name': 数据库名称,
                'backend_port': 后端端口,
                'frontend_command': 前端命令,
                'frontend_language': 前端语言,
                'backend_command': 后端命令,
                'backend_language': 后端语言
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
                        p.external_ip,
                        p.frontend_port,
                        p.workspace_path,
                        p.db_host,
                        p.db_port,
                        p.db_user,
                        p.db_password,
                        p.db_name,
                        p.backend_port,
                        p.frontend_command,
                        p.frontend_language,
                        p.backend_command,
                        p.backend_language
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
