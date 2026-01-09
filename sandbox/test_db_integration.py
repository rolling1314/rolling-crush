#!/usr/bin/env python3
"""
测试数据库集成功能
"""

import os
import requests
import json

# 沙箱服务地址
SANDBOX_URL = os.getenv("SANDBOX_URL", "http://localhost:8888")

def test_health():
    """测试健康检查"""
    print("\n1. 测试健康检查")
    print("=" * 60)
    
    response = requests.get(f"{SANDBOX_URL}/health")
    print(f"状态码: {response.status_code}")
    print(f"响应: {json.dumps(response.json(), indent=2, ensure_ascii=False)}")
    
    assert response.status_code == 200
    print("✅ 健康检查通过")


def test_execute_with_session():
    """测试带会话ID的代码执行"""
    print("\n2. 测试代码执行（带会话ID）")
    print("=" * 60)
    
    # 使用真实的会话ID（从数据库中查询）
    session_id = os.getenv("TEST_SESSION_ID", "test-session-123")
    
    # 执行简单的命令
    data = {
        "session_id": session_id,
        "command": "pwd && echo 'Hello from sandbox!'",
        "language": "bash"
    }
    
    print(f"会话ID: {session_id}")
    print(f"命令: {data['command']}")
    
    response = requests.post(
        f"{SANDBOX_URL}/execute",
        json=data,
        headers={"Content-Type": "application/json"}
    )
    
    print(f"\n状态码: {response.status_code}")
    result = response.json()
    print(f"标准输出:\n{result.get('stdout', '')}")
    print(f"标准错误:\n{result.get('stderr', '')}")
    print(f"退出码: {result.get('exit_code')}")
    
    assert response.status_code == 200
    assert result['exit_code'] == 0
    print("✅ 代码执行成功")


def test_file_operations():
    """测试文件操作"""
    print("\n3. 测试文件操作")
    print("=" * 60)
    
    session_id = os.getenv("TEST_SESSION_ID", "test-session-456")
    
    # 写入文件
    print("3.1 写入文件")
    write_data = {
        "session_id": session_id,
        "file_path": "/sandbox/test.txt",
        "content": "Hello from database integration test!\nLine 2\nLine 3"
    }
    
    response = requests.post(
        f"{SANDBOX_URL}/file/write",
        json=write_data,
        headers={"Content-Type": "application/json"}
    )
    
    print(f"状态码: {response.status_code}")
    print(f"响应: {response.json()}")
    assert response.status_code == 200
    print("✅ 文件写入成功")
    
    # 读取文件
    print("\n3.2 读取文件")
    read_data = {
        "session_id": session_id,
        "file_path": "/sandbox/test.txt"
    }
    
    response = requests.post(
        f"{SANDBOX_URL}/file/read",
        json=read_data,
        headers={"Content-Type": "application/json"}
    )
    
    print(f"状态码: {response.status_code}")
    result = response.json()
    print(f"文件内容:\n{result.get('content', '')}")
    
    assert response.status_code == 200
    assert "Hello from database integration test!" in result['content']
    print("✅ 文件读取成功")
    
    # 列出文件
    print("\n3.3 列出文件")
    list_data = {
        "session_id": session_id,
        "path": "/sandbox"
    }
    
    response = requests.post(
        f"{SANDBOX_URL}/file/list",
        json=list_data,
        headers={"Content-Type": "application/json"}
    )
    
    print(f"状态码: {response.status_code}")
    result = response.json()
    print(f"文件列表: {result.get('files', [])}")
    
    assert response.status_code == 200
    print("✅ 文件列出成功")


def test_sessions_management():
    """测试会话管理"""
    print("\n4. 测试会话管理")
    print("=" * 60)
    
    # 列出所有会话
    print("4.1 列出活跃会话")
    response = requests.get(f"{SANDBOX_URL}/sessions")
    
    print(f"状态码: {response.status_code}")
    result = response.json()
    print(f"活跃会话数: {result.get('count', 0)}")
    print(f"会话列表: {result.get('sessions', [])}")
    
    assert response.status_code == 200
    print("✅ 会话列表获取成功")


def test_database_integration():
    """测试数据库集成（需要真实的项目和会话）"""
    print("\n5. 测试数据库集成")
    print("=" * 60)
    
    # 这个测试需要在数据库中有真实的项目和会话
    # 如果设置了环境变量，则测试连接到现有容器
    
    real_session_id = os.getenv("REAL_SESSION_ID")
    if not real_session_id:
        print("⚠️ 未设置 REAL_SESSION_ID，跳过真实数据库集成测试")
        print("   提示: 设置环境变量 REAL_SESSION_ID=<真实会话ID> 进行测试")
        return
    
    print(f"使用真实会话ID: {real_session_id}")
    
    # 执行命令，应该连接到项目的容器
    data = {
        "session_id": real_session_id,
        "command": "hostname && pwd",
        "language": "bash"
    }
    
    response = requests.post(
        f"{SANDBOX_URL}/execute",
        json=data,
        headers={"Content-Type": "application/json"}
    )
    
    print(f"状态码: {response.status_code}")
    result = response.json()
    print(f"容器主机名:\n{result.get('stdout', '')}")
    
    assert response.status_code == 200
    print("✅ 数据库集成测试成功")


def main():
    """运行所有测试"""
    print("=" * 60)
    print("🧪 沙箱数据库集成测试")
    print("=" * 60)
    print(f"目标服务: {SANDBOX_URL}")
    print(f"数据库配置:")
    print(f"  POSTGRES_HOST: {os.getenv('POSTGRES_HOST', 'localhost')}")
    print(f"  POSTGRES_PORT: {os.getenv('POSTGRES_PORT', '5432')}")
    print(f"  POSTGRES_DB: {os.getenv('POSTGRES_DB', 'crush')}")
    
    try:
        test_health()
        test_execute_with_session()
        test_file_operations()
        test_sessions_management()
        test_database_integration()
        
        print("\n" + "=" * 60)
        print("✅ 所有测试通过！")
        print("=" * 60)
        
    except requests.exceptions.ConnectionError:
        print("\n❌ 无法连接到沙箱服务")
        print(f"   请确保服务正在运行: python main.py server")
        return 1
    except AssertionError as e:
        print(f"\n❌ 测试失败: {e}")
        return 1
    except Exception as e:
        print(f"\n❌ 测试异常: {e}")
        import traceback
        traceback.print_exc()
        return 1
    
    return 0


if __name__ == "__main__":
    import sys
    sys.exit(main())
