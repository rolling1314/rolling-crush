"""
自建 Docker 沙箱 - 在阿里云主机上运行
无需第三方服务，完全自托管

使用前需要在服务器上安装 Docker:
    curl -fsSL https://get.docker.com | sh
    systemctl start docker
    systemctl enable docker

安装依赖:
    pip install -r requirements.txt

配置文件:
    复制 config.example.yaml 为 config.yaml 并修改配置
    通过环境变量 SANDBOX_ENV 指定环境（development 或 production）
"""

import os
from flask import Flask
from config_loader import ConfigLoader
from database import DatabaseManager
from session_manager import SessionManager
from routes import register_routes


# 全局变量 - 延迟初始化
config = None
db_manager = None
session_manager = None


def create_app():
    """创建 Flask 应用"""
    app = Flask(__name__)
    
    # 注册所有路由
    register_routes(app)
    
    return app


def init_managers():
    """初始化配置、数据库和会话管理器（仅在服务器模式下调用）"""
    global config, db_manager, session_manager
    
    # 加载配置
    config = ConfigLoader()
    
    # 初始化数据库管理器
    db_manager = DatabaseManager(config=config)
    
    # 初始化会话管理器
    session_manager = SessionManager(db_manager=db_manager)
    
    # 将管理器存储到 app.config 中，以便在路由中访问
    app = create_app()
    app.config['config'] = config
    app.config['db_manager'] = db_manager
    app.config['session_manager'] = session_manager
    
    return app


def run_server(host=None, port=None, auto_cleanup=None):
    """运行Flask服务器
    
    Args:
        host: 监听地址（如果为 None，从配置文件读取）
        port: 监听端口（如果为 None，从配置文件读取）
        auto_cleanup: 服务器停止时是否自动清理容器（如果为 None，从配置文件读取）
    """
    # 初始化管理器并创建应用
    app = init_managers()
    
    # 从配置读取服务器参数（如果未指定）
    server_config = config.get_server_config()
    sandbox_config = config.get_sandbox_config()
    
    if host is None:
        host = server_config['host']
    if port is None:
        port = server_config['port']
    if auto_cleanup is None:
        auto_cleanup = sandbox_config.get('auto_cleanup', False)
    
    # 打印环境信息
    env = os.getenv('SANDBOX_ENV', 'development')
    print(f"🌍 运行环境: {env.upper()}", flush=True)
    print(f"🚀 沙箱服务启动在 http://{host}:{port}", flush=True)
    
    # 打印数据库连接状态
    if db_manager and db_manager.conn:
        print(f"📊 数据库: 已连接 ({db_manager.user}@{db_manager.host}:{db_manager.port}/{db_manager.database})", flush=True)
        print(f"   智能模式: 自动查询项目容器信息", flush=True)
    else:
        print(f"📊 数据库: 未连接，运行在独立模式", flush=True)
    
    print(f"\n📝 API端点:", flush=True)
    print(f"   健康检查:", flush=True)
    print(f"   - GET  /health          健康检查", flush=True)
    print(f"   - GET  /sessions        列出会话", flush=True)
    print(f"   - POST /sessions/cleanup 清理所有会话", flush=True)
    print(f"   - DELETE /session/<id>  删除会话", flush=True)
    print(f"\n   代码执行:", flush=True)
    print(f"   - POST /execute         执行命令", flush=True)
    print(f"   - POST /diagnostic      获取诊断信息", flush=True)
    print(f"\n   LSP服务:", flush=True)
    print(f"   - POST /lsp/diagnostics LSP诊断信息", flush=True)
    print(f"\n   文件操作:", flush=True)
    print(f"   - POST /file/read       读取文件", flush=True)
    print(f"   - POST /file/write      写入文件", flush=True)
    print(f"   - POST /file/list       列出文件", flush=True)
    print(f"   - POST /file/grep       搜索内容", flush=True)
    print(f"   - POST /file/glob       搜索文件名", flush=True)
    print(f"   - POST /file/edit       编辑文件", flush=True)
    print(f"   - GET  /file/tree       获取文件树", flush=True)
    print(f"\n   项目管理:", flush=True)
    print(f"   - POST /projects/create 创建项目容器", flush=True)
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
