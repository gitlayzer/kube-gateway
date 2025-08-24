# Kube-Gateway

**一个集中的、零配置的 Kubernetes API 网关，旨在简化多集群管理。**

`Kube-Gateway` 提供一个统一、安全的 API 入口，让开发者和运维人员可以使用单一的、自动管理的 `kubeconfig` 文件，无缝地访问和操作所有已授权的 Kubernetes 集群。

## 目录

- [背景](#背景)
- [核心特性](#核心特性)
- [架构简图](#架构简图)
- [安装](#安装)
- [命令详解](#命令详解)

## 背景

随着云原生技术的发展，在开发和生产环境中使用数十个甚至上百个 Kubernetes 集群已成为常态。管理这些集群的访问凭证 (`kubeconfig`) 变得日益复杂和混乱：
- 开发者需要在本地维护大量配置文件。
- 轮换密钥和更新凭证成为一项繁重且易错的任务。
- 无法对所有集群的访问进行统一的审计和控制。

`Kube-Gateway` 的诞生就是为了解决这些痛点。

## 核心特性

- **🚀 统一访问入口**: 所有 `kubectl` 请求都指向同一个网关地址，由网关智能路由到后端对应的集群。
- **⚙️ 零配置启动**: 首次启动服务时，自动生成所需的 TLS 证书，无需任何手动 `openssl` 操作。
- **🤝 客户端无缝集成**: `add` 和 `remove` 命令会自动、安全地更新你本地的 `~/.kube/config` 文件，包括备份和恢复。
- **⚡️ 零停机热加载**: 添加或删除集群后，只需执行 `reload` 命令即可动态更新服务配置，API 服务全程不中断。
- **🛠️ 强大的命令行工具链**: 使用 `cobra` 构建了完整、易用的 CLI，覆盖了从服务管理到配置的所有方面。
- **🩺 集群健康探测**: 内置 `health` 命令，可并发检查所有纳管集群的连通性、K8s 版本和 API 延迟。
- **🎯 直接命令代理**: 独创 `exec` 命令，无需切换上下文，即可在指定集群上快速执行任何 `kubectl` 或 `helm` 命令。
- **🔑 凭证安全轮换**: 内置 `token rotate` 命令，允许管理员一键为指定集群生成新 Token 并自动更新客户端配置，提升安全性。
- **📜 详细审计日志**: 可选地将所有通过网关的 API 请求以 JSON 格式记录到文件中，用于安全审计与合规。
- **🔒 默认安全**: 强制使用 HTTPS，并自动为客户端配置 CA 信任，避免不安全的连接。

## 架构简图

`Kube-Gateway` 的工作流程非常简单直接：

![image.png](https://image.devops-engineer.com.cn/file/1756026304579_image.png)


1. **认证请求**: 用户的 `kubectl` 使用包含特定 Token 的 `kubeconfig` 连接到 `Kube-Gateway`。
2. **网关处理**: 网关验证 Token，并根据 Token 找到对应的后端集群。
3. **安全转发**: 网关使用自己持有的凭证，将请求安全地转发给真正的 K8s 集群。

## 安装

### 使用预编译的二进制文件

前往本项目的 [Releases 页面](https://github.com/gitlayzer/kube-gateway/releases) 下载适用于你操作系统的最新版本。

## 命令详解
```bash
serve
启动 API 网关主服务。

kube-gateway serve

标志 (Flags):
--enable-audit-log: (可选) 启用 API 请求的审计日志功能。日志将以 JSON 格式记录在 ~/.kube-gateway/logs/audit.log 文件中。
--public-address=<ip-or-domain>: (可选) 指定一个公共 IP 或域名。此地址将被添加到自签名 TLS 证书中，以便团队成员可以远程访问。默认为 127.0.0.1。
```

```bash
add <集群名称> <kubeconfig路径>
添加一个新的集群配置，并自动更新本地 ~/.kube/config。

kube-gateway add my-cluster /path/to/my-cluster.config
```

```bash
list
以表格形式列出所有已由 kube-gateway 管理的集群及其详细信息。

kube-gateway list
```

```bash
remove <集群名称>
移除一个集群配置，并自动清理本地 ~/.kube/config 中相关的条目。

kube-gateway remove my-cluster
```

```bash
reload
通知正在运行的 serve 进程热加载最新的集群配置，服务不中断。

kube-gateway reload
```

```bash
health
并发检查所有已配置集群的 API Server 连通性、K8s 版本和延迟。

kube-gateway health
```

```bash
exec <集群名称> -- <命令...>
在指定集群上执行任意命令，而无需切换本地的 kubectl 上下文。使用 -- 来分隔 exec 命令和要执行的命令。

kube-gateway exec dev -- kubectl get pods

kube-gateway exec staging -- helm list -n default
```

```bash
token rotate <集群名称>
为指定的集群生成一个新的认证 Token，并自动更新服务端和客户端的配置。

kube-gateway token rotate dev
```