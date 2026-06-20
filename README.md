# TermBridge-go

TermBridge-go 是 TermBridge 的 Go 重写版本，一个本地浏览器工作区，用于管理终端会话。

## 设计文档

- [设计文档](docs/design.md) - 项目背景、愿景、架构设计、技术路线

## 开发要求

### 开发依赖

- Go 1.25+
- [just](https://github.com/casey/just)
- [Air](https://github.com/air-verse/air)

## 开发方式

### 后端热加载

```bash
just backend
```

### 前端开发服务

```bash
just frontend
```

## 使用方式

### 在当前目录运行命令

```powershell
termbridge exec -- claude
```

### 在指定目录运行命令

```powershell
termbridge --cwd D:\project exec -- claude
```

### 查看本地会话和工作区

```powershell
termbridge session
termbridge workspace
```

### 启动本地 Web workbench

```powershell
termbridge web --dev
```
