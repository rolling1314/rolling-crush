# Sandbox 项目结构

重构后的目录结构如下：

```
sandbox/
├── main.py                 # 应用入口和服务器启动
├── database.py            # 数据库管理器
├── sandbox.py             # Docker 沙箱核心类
├── session_manager.py     # 会话管理器
├── routes/                # 路由模块
│   ├── __init__.py       # 路由注册
│   ├── health.py         # 健康检查和会话管理路由
│   ├── execute.py        # 代码执行路由
│   ├── file_ops.py       # 文件操作路由
│   └── project.py        # 项目管理路由
├── requirements.txt       # Python 依赖
├── start.sh              # 启动脚本
└── config.example.sh     # 配置示例
```

## 模块说明

### 核心模块

- **database.py**: PostgreSQL 数据库管理，查询会话和项目信息
- **sandbox.py**: 基于 Docker 的代码沙箱实现
- **session_manager.py**: 管理会话到沙箱容器的映射关系

### 路由模块

- **health.py**: 
  - `GET /health` - 健康检查
  - `GET /sessions` - 列出活跃会话
  - `POST /sessions/cleanup` - 清理所有会话
  - `DELETE /session/<id>` - 删除指定会话

- **execute.py**:
  - `POST /execute` - 执行代码
  - `POST /diagnostic` - 获取诊断信息

- **file_ops.py**:
  - `POST /file/read` - 读取文件
  - `POST /file/write` - 写入文件
  - `POST /file/list` - 列出文件
  - `POST /file/grep` - 搜索内容
  - `POST /file/glob` - 搜索文件名
  - `POST /file/edit` - 编辑文件
  - `GET /file/tree` - 获取文件树

- **project.py**:
  - `POST /projects/create` - 创建项目容器

## 启动方式

```bash
python main.py
```

或使用启动脚本：

```bash
./start.sh
```
