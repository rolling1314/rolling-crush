"""
路由模块初始化
"""

from flask import Flask
from routes.health import health_bp
from routes.execute import execute_bp
from routes.file_ops import file_ops_bp
from routes.project import project_bp
from routes.lsp import lsp_bp


def register_routes(app: Flask):
    """注册所有路由蓝图"""
    app.register_blueprint(health_bp)
    app.register_blueprint(execute_bp)
    app.register_blueprint(file_ops_bp)
    app.register_blueprint(project_bp)
    app.register_blueprint(lsp_bp)