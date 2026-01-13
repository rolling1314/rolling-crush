"""
配置加载器 - 从 YAML 文件加载配置
支持多环境配置（development, production）
"""

import os
import yaml
from typing import Dict, Any


class ConfigLoader:
    """配置加载器"""
    
    def __init__(self, config_file: str = "config.yaml"):
        """初始化配置加载器
        
        Args:
            config_file: 配置文件路径，默认为 config.yaml
        """
        self.config_file = config_file
        self.env = os.getenv("SANDBOX_ENV", "development")
        self.config = self._load_config()
    
    def _load_config(self) -> Dict[str, Any]:
        """加载配置文件"""
        try:
            # 获取当前文件所在目录
            current_dir = os.path.dirname(os.path.abspath(__file__))
            config_path = os.path.join(current_dir, self.config_file)
            
            # 检查配置文件是否存在
            if not os.path.exists(config_path):
                print(f"⚠️  配置文件不存在: {config_path}")
                print(f"   将使用环境变量或默认配置")
                return self._get_default_config()
            
            # 读取 YAML 配置文件
            with open(config_path, 'r', encoding='utf-8') as f:
                config = yaml.safe_load(f)
            
            # 获取当前环境的配置
            if self.env not in config:
                print(f"⚠️  配置文件中未找到环境 '{self.env}'，使用默认配置")
                return self._get_default_config()
            
            env_config = config[self.env]
            print(f"✅ 已加载配置: {self.config_file} (环境: {self.env})")
            return env_config
            
        except Exception as e:
            print(f"⚠️  加载配置文件失败: {e}")
            print(f"   将使用环境变量或默认配置")
            return self._get_default_config()
    
    def _get_default_config(self) -> Dict[str, Any]:
        """获取默认配置（兜底）"""
        return {
            'server': {
                'host': '0.0.0.0',
                'port': 8888,
                'debug': False
            },
            'database': {
                'host': 'localhost',
                'port': 5432,
                'user': 'crush',
                'password': '123456',
                'database': 'crush',
                'sslmode': 'disable'
            },
            'sandbox': {
                'auto_cleanup': False,
                'session_timeout': 3600
            }
        }
    
    def get(self, key: str, default: Any = None) -> Any:
        """获取配置值（支持点号分隔的嵌套键）
        
        Args:
            key: 配置键，支持点号分隔，如 'database.host'
            default: 默认值
        
        Returns:
            配置值
        """
        keys = key.split('.')
        value = self.config
        
        for k in keys:
            if isinstance(value, dict) and k in value:
                value = value[k]
            else:
                return default
        
        return value
    
    def get_database_config(self) -> Dict[str, Any]:
        """获取数据库配置（优先使用环境变量）"""
        db_config = self.config.get('database', {})
        
        # 环境变量优先级更高
        return {
            'host': os.getenv('POSTGRES_HOST', db_config.get('host', 'localhost')),
            'port': int(os.getenv('POSTGRES_PORT', db_config.get('port', 5432))),
            'user': os.getenv('POSTGRES_USER', db_config.get('user', 'crush')),
            'password': os.getenv('POSTGRES_PASSWORD', db_config.get('password', '')),
            'database': os.getenv('POSTGRES_DB', db_config.get('database', 'crush')),
            'sslmode': os.getenv('POSTGRES_SSLMODE', db_config.get('sslmode', 'disable'))
        }
    
    def get_server_config(self) -> Dict[str, Any]:
        """获取服务器配置"""
        server_config = self.config.get('server', {})
        
        return {
            'host': os.getenv('SERVER_HOST', server_config.get('host', '0.0.0.0')),
            'port': int(os.getenv('SERVER_PORT', server_config.get('port', 8888))),
            'debug': server_config.get('debug', False)
        }
    
    def get_sandbox_config(self) -> Dict[str, Any]:
        """获取沙箱配置"""
        return self.config.get('sandbox', {
            'auto_cleanup': False,
            'session_timeout': 3600
        })
