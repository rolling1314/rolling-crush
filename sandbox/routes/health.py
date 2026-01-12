"""
健康检查和会话管理路由
"""

from flask import Blueprint, jsonify, current_app

health_bp = Blueprint('health', __name__)


@health_bp.route('/health', methods=['GET'])
def health():
    """健康检查"""
    session_manager = current_app.config.get('session_manager')
    active_sessions = len(session_manager.sessions) if session_manager else 0
    return jsonify({"status": "ok", "active_sessions": active_sessions})


@health_bp.route('/sessions', methods=['GET'])
def list_sessions():
    """列出所有活跃会话"""
    session_manager = current_app.config.get('session_manager')
    sessions = session_manager.list_sessions()
    print(f"\n📨 [GET /sessions] 查询活跃会话")
    print(f"   活跃会话数: {len(sessions)}")
    return jsonify({
        "sessions": sessions,
        "count": len(sessions)
    })


@health_bp.route('/sessions/cleanup', methods=['POST'])
def cleanup_all_sessions():
    """清理所有会话和容器"""
    session_manager = current_app.config.get('session_manager')
    print(f"\n📨 [POST /sessions/cleanup] 收到清理请求")
    count = len(session_manager.sessions)
    session_manager.cleanup_all()
    print(f"✅ [POST /sessions/cleanup] 已清理 {count} 个会话")
    return jsonify({
        "status": "ok",
        "message": f"Cleaned up {count} sessions"
    })


@health_bp.route('/session/<session_id>', methods=['DELETE'])
def delete_session(session_id):
    """删除会话和对应的容器"""
    session_manager = current_app.config.get('session_manager')
    print(f"\n📨 [DELETE /session] 收到删除请求")
    print(f"   会话ID: {session_id}")
    
    session = session_manager.get(session_id)
    if session:
        session_manager.remove(session_id)
        print(f"✅ [DELETE /session] 会话已删除")
        return jsonify({"status": "ok", "message": f"Session {session_id} removed"})
    else:
        print(f"⚠️ [DELETE /session] 会话不存在")
        return jsonify({"status": "ok", "message": f"Session {session_id} not found"})
