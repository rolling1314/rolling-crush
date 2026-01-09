# Working Directory 功能测试指南

## 前置条件

1. 确保数据库中的 `projects` 表有正确的 `workdir_path` 值
2. 确保 `sessions` 表中的记录正确关联到 `project_id`

## 测试场景

### 场景 1: 验证项目工作目录被正确使用

#### 准备数据

```sql
-- 创建测试项目
INSERT INTO projects (id, user_id, name, workdir_path, created_at, updated_at)
VALUES ('test-project-1', 'user-1', 'Test Project', '/path/to/project1', 
        EXTRACT(EPOCH FROM NOW()) * 1000, EXTRACT(EPOCH FROM NOW()) * 1000);

-- 创建关联的会话
INSERT INTO sessions (id, project_id, title, created_at, updated_at)
VALUES ('test-session-1', 'test-project-1', 'Test Session',
        EXTRACT(EPOCH FROM NOW()) * 1000, EXTRACT(EPOCH FROM NOW()) * 1000);
```

#### 测试步骤

1. 启动应用
2. 使用 session ID `test-session-1` 发送一个命令,例如:
   ```
   请使用 bash 工具执行 pwd 命令
   ```
3. 检查日志输出,应该能看到:
   ```
   Using project-specific working directory session_id=test-session-1 project_id=test-project-1 workdir=/path/to/project1
   ```
4. 检查 bash 工具返回的 working directory 应该是 `/path/to/project1`

### 场景 2: 多个项目使用不同的工作目录

#### 准备数据

```sql
-- 项目 A
INSERT INTO projects (id, user_id, name, workdir_path, created_at, updated_at)
VALUES ('project-a', 'user-1', 'Project A', '/path/to/projectA',
        EXTRACT(EPOCH FROM NOW()) * 1000, EXTRACT(EPOCH FROM NOW()) * 1000);

INSERT INTO sessions (id, project_id, title, created_at, updated_at)
VALUES ('session-a', 'project-a', 'Session A',
        EXTRACT(EPOCH FROM NOW()) * 1000, EXTRACT(EPOCH FROM NOW()) * 1000);

-- 项目 B
INSERT INTO projects (id, user_id, name, workdir_path, created_at, updated_at)
VALUES ('project-b', 'user-1', 'Project B', '/path/to/projectB',
        EXTRACT(EPOCH FROM NOW()) * 1000, EXTRACT(EPOCH FROM NOW()) * 1000);

INSERT INTO sessions (id, project_id, title, created_at, updated_at)
VALUES ('session-b', 'project-b', 'Session B',
        EXTRACT(EPOCH FROM NOW()) * 1000, EXTRACT(EPOCH FROM NOW()) * 1000);
```

#### 测试步骤

1. 使用 `session-a` 执行命令,验证使用 `/path/to/projectA`
2. 使用 `session-b` 执行命令,验证使用 `/path/to/projectB`
3. 交替使用两个 session,确保工作目录不会混淆

### 场景 3: 没有关联项目的会话使用默认目录

#### 测试步骤

1. 创建一个没有 project_id 的 session:
   ```sql
   INSERT INTO sessions (id, project_id, title, created_at, updated_at)
   VALUES ('session-no-project', NULL, 'No Project Session',
           EXTRACT(EPOCH FROM NOW()) * 1000, EXTRACT(EPOCH FROM NOW()) * 1000);
   ```
2. 使用这个 session 执行命令
3. 验证工具使用配置文件中的默认 working directory

### 场景 4: 测试各种工具

测试以下工具都能正确使用项目工作目录:

1. **bash 工具**: `pwd` 命令应该显示项目目录
2. **ls 工具**: 列出项目根目录内容
3. **view 工具**: 读取项目中的相对路径文件
4. **edit 工具**: 编辑项目中的文件
5. **write 工具**: 在项目目录中创建新文件
6. **glob 工具**: 在项目目录中查找文件
7. **grep 工具**: 在项目目录中搜索内容

## 日志验证

在应用日志中查找以下信息:

```
INFO Using project-specific working directory
  session_id: <session-id>
  project_id: <project-id>
  workdir: <working-directory-path>
```

如果看到警告:
```
WARN Failed to get session for workdir lookup
WARN Failed to get project for workdir lookup
```

说明数据库查询失败,需要检查:
1. Session 是否存在
2. Project 是否存在
3. Session 是否正确关联到 Project

## 常见问题

### Q: 工具仍然使用旧的工作目录?

A: 检查以下几点:
1. 确认代码已重新编译并重启应用
2. 检查数据库中 `workdir_path` 是否正确设置
3. 检查日志是否有相关警告信息

### Q: 如何验证功能是否正常工作?

A: 最简单的方法是:
1. 在日志中查找 "Using project-specific working directory" 消息
2. 使用 bash 工具执行 `pwd` 命令,检查返回的路径
3. 比较不同项目的 session,确保工作目录不同

### Q: 工具参数中指定的 working_dir 还有效吗?

A: 是的!优先级如下:
1. 工具参数中明确指定的 `working_dir` (最高优先级)
2. Context 中的项目工作目录
3. 工具初始化时的默认工作目录 (最低优先级)

## 性能考虑

每次请求会执行两次数据库查询:
1. 查询 session 信息
2. 查询 project 信息 (如果 session 有 project_id)

这些查询是轻量级的,不会显著影响性能。如果需要优化,可以考虑:
1. 在 session 表中添加 `workdir_path` 冗余字段
2. 使用缓存机制缓存 project 信息
