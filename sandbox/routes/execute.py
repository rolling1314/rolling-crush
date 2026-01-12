"""
代码执行路由
"""

import docker
import traceback
from flask import Blueprint, request, jsonify, current_app

execute_bp = Blueprint('execute', __name__)


@execute_bp.route('/execute', methods=['POST'])
def execute_code():
    """执行代码 - 对应 bash 工具"""
    try:
        session_manager = current_app.config.get('session_manager')
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
        traceback.print_exc()
        return jsonify({"error": f"内部错误: {str(e)}"}), 500


@execute_bp.route('/diagnostic', methods=['POST'])
def get_diagnostics():
    """获取诊断信息 - 对应 diagnostics 工具"""
    try:
        session_manager = current_app.config.get('session_manager')
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
