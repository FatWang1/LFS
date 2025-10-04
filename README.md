# LFS - Local File Storage

🚀 **高性能大文件存储服务** - 支持分片上传、断点续传、完整性校验的单可执行文件服务

## ✨ 特性

### 🎯 核心功能
- **大文件分片上传** - 支持超大文件的分片传输
- **断点续传** - 网络中断后可继续上传
- **完整性校验** - MD5校验确保文件完整性
- **批量操作** - 支持批量上传和下载
- **静态文件嵌入** - 前端完全打包到可执行文件中

### ⚡ 性能优化
- **HTTP/2支持** - 多路复用，减少连接开销
- **Gzip压缩** - 约73%压缩率，减少传输时间
- **智能缓存** - 静态文件内存缓存，零延迟响应
- **连接池** - 支持100并发上传，200并发下载
- **大缓冲区** - 4MB缓冲区，提升传输速度

### 🛡️ 安全特性
- **CORS支持** - 跨域访问控制
- **安全头** - XSS保护、内容类型检查
- **路径验证** - 防止路径遍历攻击
- **超时控制** - 防止资源泄露

## 🚀 快速开始

### 构建和运行

```bash
# 克隆项目
git clone <repository-url>
cd LFS

# 构建（生成单个可执行文件）
go build -o lfs-server .

# 运行
./lfs-server
```

服务将在 `http://localhost:8080` 启动

### 环境变量

```bash
# 设置存储路径（可选，默认为 /tmp/）
export STORAGE_PATH=/path/to/storage

# 运行服务
./lfs-server
```

## 📡 API 接口

### 文件上传
```bash
# 单文件上传
curl -X POST -F "file=@example.txt" http://localhost:8080/upload

# 分片上传
curl -X POST -F "file=@chunk.bin" \
  -F "chunkIndex=0" \
  -F "chunkSize=5242880" \
  -F "totalChunk=10" \
  -F "md5=abc123" \
  http://localhost:8080/upload-chunk

# 批量上传
curl -X POST -F "files=@file1.txt" -F "files=@file2.txt" \
  http://localhost:8080/batch-upload
```

### 文件下载
```bash
# 单文件下载
curl -O http://localhost:8080/download/example.txt

# 分片下载
curl "http://localhost:8080/download-chunk/example.txt?chunkIndex=0&chunkSize=5242880"

# 批量下载
curl "http://localhost:8080/batch-download?filenames=file1.txt,file2.txt"
```

### 文件管理
```bash
# 列出文件
curl http://localhost:8080/files

# 获取文件MD5
curl http://localhost:8080/file-md5/example.txt

# 性能监控
curl http://localhost:8080/metrics
```

## 🏗️ 项目结构

```
LFS/
├── main.go                 # 主程序入口
├── config/                 # 配置管理
│   └── config.go
├── handlers/               # HTTP处理器
│   └── file_handlers.go
├── storage/                # 文件存储
│   └── file_storage.go
├── optimization/           # 性能优化
│   └── performance.go
├── static/                 # 静态文件（嵌入）
│   ├── index.html
│   ├── style.css
│   └── script.js
└── README.md
```

## 🔧 技术栈

- **后端**: Go 1.21+, Gin Web框架
- **前端**: HTML5, CSS3, JavaScript (ES6+)
- **协议**: HTTP/1.1, HTTP/2
- **压缩**: Gzip
- **校验**: MD5
- **并发**: Goroutines + Channels

## 📊 性能指标

- **响应时间**: 主页 < 100µs
- **压缩率**: CSS文件 73% (6902→1841字节)
- **并发支持**: 100上传 + 200下载
- **缓冲区**: 4MB (文件传输), 2MB (分片)
- **内存使用**: 智能缓存 + 连接池

## 🎨 前端界面

访问 `http://localhost:8080` 使用现代化的Web界面：

- 📁 拖拽上传
- 📊 实时进度显示
- 📋 文件列表管理
- 🔄 断点续传支持
- ✅ 完整性校验

## 🚀 部署

### 单机部署
```bash
# 直接运行
./lfs-server

# 后台运行
nohup ./lfs-server > lfs.log 2>&1 &
```

### Docker部署（可选）
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o lfs-server .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/lfs-server .
COPY --from=builder /app/static ./static
EXPOSE 8080
CMD ["./lfs-server"]
```

## 🔍 监控

访问 `http://localhost:8080/metrics` 查看实时性能数据：

```json
{
  "memory": {
    "alloc": 1346264,
    "total_alloc": 5255264,
    "sys": 12865552,
    "num_gc": 2
  },
  "runtime": {
    "goroutines": 3,
    "cpu_cores": 10,
    "max_procs": 10
  },
  "cache": {
    "static_files": 3
  }
}
```

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT License

---

**LFS** - 让本地网络传输变得简单高效！ 🚀