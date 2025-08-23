# Kube-Gateway

![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go)
![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen?style=for-the-badge)

**一个集中的、零配置的 Kubernetes API 网关，旨在简化多集群管理。**

---

`Kube-Gateway` 提供一个统一、安全的 API 入口，让开发者和运维人员可以使用单一的、自动管理的 `kubeconfig` 文件，无缝地访问和操作所有已授权的 Kubernetes 集群。

## 目录

- [背景](#背景)
- [核心特性](#核心特性)
- [架构简图](#架构简图)
- [安装](#安装)
- [详细使用与测试指南 (Linux)](#详细使用与测试指南-linux)
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
- **🔒 默认安全**: 强制使用 HTTPS，并自动为客户端配置 CA 信任，避免不安全的连接。

## 架构简图

`Kube-Gateway` 的工作流程非常简单直接：

```
+-----------+         +----------------------------+         +---------------------+
|           |         |                            |         |                     |
|  开发者   |--(1)--> |   Kube-Gateway (HTTPS)     |--(3)--> |  后端 K8s 集群 A   |
| (kubectl) |         |                            |         |                     |
+-----------+         |  - 认证 (检查 Token)       |         +---------------------+
                      |  - 路由 (查找对应集群)     |
                      |  - 代理 (转发请求)         |         +---------------------+
                      |                            |         |                     |
                      +----------------------------+--(4)--> |  后端 K8s 集群 B   |
                                                             |                     |
                                                             +---------------------+
```
1.  **认证请求**: 用户的 `kubectl` 使用包含特定 Token 的 `kubeconfig` 连接到 `Kube-Gateway`。
2.  **网关处理**: 网关验证 Token，并根据 Token 找到对应的后端集群。
3.  **安全转发**: 网关使用自己持有的凭证，将请求安全地转发给真正的 K8s 集群。

## 安装

### 使用预编译的二进制文件
前往本项目的 [Releases 页面](https://github.com/gitlayzer/kube-gateway/releases) 下载适用于你操作系统的最新版本。

## 详细使用与测试指南 (Linux)

本指南将从一个纯净的 Linux 环境开始，带你完成 `kube-gateway` 的编译、部署和所有核心功能的端到端测试。

### 准备工作

#### 1. 环境要求
- `Go` (1.24 或更高版本)
- `Docker`
- `kubectl`
- `kind`

#### 2. 编译项目
在项目根目录下执行编译。
```bash
# 克隆仓库
git clone https://github.com/gitlayzer/kube-gateway.git

# 或者直接使用 go build
go build -o kube-gateway .

# 确保二进制文件有执行权限
chmod +x ./kube-gateway
```

#### 3. 创建本地测试集群
我们将使用 `kind` 创建三个临时的 Kubernetes 集群用于测试。
```bash
# 创建 dev, staging, prod 三个集群
kind create cluster --name dev
kind create cluster --name staging
kind create cluster --name prod

# 导出它们的 kubeconfig 文件到主目录
kind get kubeconfig --name dev > ~/dev.config
kind get kubeconfig --name staging > ~/staging.config
kind get kubeconfig --name prod > ~/prod.config
```

### 端到端测试流程

#### 步骤 1: 首次启动与基础验证

1.  **启动服务**
    打开 **终端1**，在后台启动 `kube-gateway` 服务，并将日志输出到 `gateway.log`。
    ```bash
    nohup ./kube-gateway serve > gateway.log 2>&1 &
    ```

2.  **验证服务状态**
    检查日志和自动生成的目录。
    ```bash
    # 查看实时日志
    tail -f gateway.log
    ```
    **预期结果**: 你应该能看到日志显示服务正在自动生成 TLS 证书，并成功启动。
    ```
    未找到 TLS 证书，正在自动生成新的证书...
    ✅ 成功生成并保存证书到 /home/user/.kube-gateway/certs/server.pem ...
    配置加载完毕。当前有 0 个集群代理处于活动状态。
    正在启动 kube-gateway HTTPS 服务器于 0.0.0.0:8443 (PID: 12345)
    ```
    使用 `ls -R ~/.kube-gateway` 可以看到 `certs` 和 `pid` 目录已被创建。

#### 步骤 2: 添加并激活第一个集群

1.  **测试 `add` 命令**
    在 **终端2** 中添加 `dev` 集群。
    ```bash
    ./kube-gateway add dev ~/dev.config
    ```
    **预期结果**: 提示服务端配置成功，并自动更新了本地 `~/.kube/config`。

2.  **测试 `list` 命令**
    ```bash
    ./kube-gateway list
    ```
    **预期结果**: 列出 `dev` 集群和部分 Token 信息。
    ```
    CONFIGURED CLUSTERS:
      - dev (Token: ...xxxxxxxx)
    ```

3.  **测试 `kubectl` (热加载前)**
    此时服务内存中还没有新集群的配置，请求应该会失败。
    ```bash
    kubectl get nodes
    ```
    **预期结果**: **失败**，并报错 `401 Unauthorized`。

4.  **测试 `reload` 命令**
    通知服务加载新配置。
    ```bash
    ./kube-gateway reload
    ```
    **预期结果**: 提示 `SIGHUP` 信号已发送。回到 **终端1** 查看 `gateway.log`，应该能看到日志显示“收到 SIGHUP 信号”并且“1 个集群代理处于活动状态”。

5.  **测试 `kubectl` (热加载后)**
    再次尝试访问。
    ```bash
    kubectl get nodes
    ```
    **预期结果**: **成功**，并显示 `dev-control-plane` 节点的信息。

#### 步骤 3: 管理多个集群

1.  **添加第二个集群**
    在 **终端2** 中添加 `staging` 集群并重载。
    ```bash
    ./kube-gateway add staging ~/staging.config
    ./kube-gateway reload
    ```

2.  **验证 `list` 命令**
    ```bash
    ./kube-gateway list
    ```
    **预期结果**: `dev` 和 `staging` 两个集群都被列出。

3.  **验证多集群访问**
    使用 `kubectl` 切换上下文并访问 `staging` 集群。
    ```bash
    # kubectl 会自动知道这个新上下文
    kubectl config use-context gateway-staging
    kubectl get nodes
    ```
    **预期结果**: **成功**，并显示 `staging-control-plane` 节点的信息。

#### 步骤 4: 移除集群

1.  **测试 `remove` 命令**
    我们将移除 `dev` 集群。
    ```bash
    ./kube-gateway remove dev
    ```
    **预期结果**: 提示服务端配置已移除，并自动清理了本地 `~/.kube/config`。

2.  **验证 `list` 和 `kubectl`**
    ```bash
    # list 命令中应该只剩下 staging
    ./kube-gateway list

    # 尝试查看已删除的 dev 上下文
    kubectl config get-contexts | grep gateway-dev
    ```
    **预期结果**: `list` 命令只显示 `staging` 集群。`get-contexts` 命令找不到 `gateway-dev` 上下文。

3.  **测试热加载**
    ```bash
    ./kube-gateway reload
    ```
    查看 **终端1** 的日志，应该能看到“1 个集群代理处于活动状态”。

4.  **最终验证**
    访问 `staging` 集群应该仍然正常。
    ```bash
    kubectl get nodes --context=gateway-staging
    ```
    **预期结果**: **成功**。这证明移除一个集群不会影响其他集群。

#### 步骤 5: 清理工作

测试完成后，清理所有资源。

1.  **停止服务**
    ```bash
    # 读取 PID 文件并停止进程
    kill $(cat ~/.kube-gateway/pid/kube-gateway.pid)
    ```
2.  **删除 `kind` 集群**
    ```bash
    kind delete clusters dev staging prod
    ```
3.  **删除所有配置文件**
    ```bash
    rm ~/dev.config ~/staging.config ~/prod.config
    rm -rf ~/.kube-gateway
    # 别忘了恢复你之前的 kubeconfig (如果有备份)
    # mv ~/.kube/config.bak ~/.kube/config
    ```

## 命令详解

#### `serve`
启动 API 网关主服务。
```bash
kube-gateway serve
```

#### `add <集群名称> <kubeconfig路径>`
添加一个新的集群配置，并自动更新本地 `~/.kube/config`。
```bash
kube-gateway add my-cluster /path/to/my-cluster.config
```

#### `list`
列出所有已由 `kube-gateway` 管理的集群。
```bash
kube-gateway list
```

#### `remove <集群名称>`
移除一个集群配置，并自动清理本地 `~/.kube/config` 中相关的条目。
```bash
kube-gateway remove my-cluster
```

#### `reload`
通知正在运行的 `serve` 进程热加载最新的集群配置，服务不中断。
```bash
kube-gateway reload
```

## 建议
在生产环境的 Linux 服务器上，建议使用 `systemd` 来管理服务。