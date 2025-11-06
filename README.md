# Gomate - 远程文件编辑客户端

## Gomate 🚀

Gomate 是一个轻量级、健壮的跨平台命令行工具，用于通过网络连接将本地文件发送到远程编辑器（如 Sublime Text 的 `remote_subl` 插件）进行编辑。

它优雅地处理了 Windows 命令行启动链、多实例互斥和文件生命周期管理。

## ✨ 主要特性

- **一键安装**: 脚本自动下载最新 `gomate.exe` 并完成所有配置。
- **灵活配置**：支持命令行参数、环境变量、默认值三级优先级配置主机和端口。
- **后台启动**：通过 VBScript 隐藏窗口启动，不占用命令行窗口，即时返回。
- **零残留**: 核心程序（gomate.exe）在文件关闭后立即且干净地退出，无残留进程。
- **文件创建**：如果编辑的文件不存在，程序会自动创建文件及其所需的多级父目录。
- **互斥锁定**：通过全局锁文件机制，确保同一时间只有一个客户端实例编辑同一个文件，防止数据冲突和进程残留。
- **调试日志**：通过 `-v` / `-verbose` 参数控制详细日志输出。

## 📦 一键部署 (无需 Go 环境)

对于没有 Go 环境的用户，可以使用 PowerShell 脚本从 GitHub 仓库直接下载并部署最新的 `gomate.exe` 和支持文件。

### 可选部署参数

安装脚本支持通过参数自定义安装位置和默认连接配置：

| **参数**      | **默认值**      | **描述**                                |
| ------------- | -------------- | --------------------------------------- |
| `-InstallDir` | `C:\SW\gomate` | Gomate 启动脚本和核心文件存放的根目录。 |
| `-hostname`   | `localhost`    | 设置默认的系统环境变量 `GOMATE_HOST`。  |
| `-port`       | `52698`        | 设置默认的系统环境变量 `GOMATE_PORT`。  |

> 请将 `{WiseScripts}` 和 `{gomate}` 替换为您的实际仓库信息。
> 
> ❗ 必须以管理员身份运行 PowerShell

请在 **PowerShell** 中执行以下命令：

### 默认安装 (安装到 `C:\SW\gomate`)

```PowerShell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/WiseScripts/gomate/refs/heads/main/install-gomate.ps1)))
```

### 自定义安装目录

使用 `-InstallDir` 参数指定安装路径，例如 `C:\Tools\gomate`

```PowerShell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/WiseScripts/gomate/refs/heads/main/install-gomate.ps1))) -InstallDir C:\Tools\gomate
```

### 自定义远程主机名

您可以在安装时直接设置默认的远程主机名和端口

```PowerShell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/WiseScripts/gomate/refs/heads/main/install-gomate.ps1))) -hostname 1.2.3.4 -port 52698
```

> **注意：** 脚本执行完毕后，安装目录已经添加到系统的 **`PATH` 环境变量**中，重启开启CMD后可以在任何位置运行 `gomate.cmd`。

### 部署细节与文件结构

默认安装路径 (`C:\SW\gomate`) 下的文件结构如下：

```
C:\SW\gomate\
├── gomate.cmd        <-- 启动入口，位于 PATH 优先目录。
└── core\
    ├── gomate.exe    <-- 核心 Go 程序。
    └── gomate.ps1    <-- 启动逻辑。
```

------

## 💻 使用方法

部署完成后，在您的远程 SSH 会话中，无论是通过 **CMD** 还是 **PowerShell**，您都可以直接使用 `gomate` 命令进行编辑。

```Bash
# 示例：打开文件，Shell 立即返回
gomate C:\path\to\file.txt
```

### Gomate 内部工作流

| **组件**                    | **作用**                                                     |
| --------------------------- | ------------------------------------------------------------ |
| **`gomate.exe`** (Core)     | 核心网络程序，在文件关闭后干净退出。       |
| **`gomate.vbs`** (Launcher) | 隐藏窗口启动核心程序，实现进程隔离。       |
| **`gomate.cmd`** (Entry)    | 负责参数传递和调用 `gomate.vbs`，并通过 `PATH` 优先级确保 `gomate` 命令始终指向它。 |

### 连接参数优先级

1. **命令行参数** (`--hostname` / `--port`)：最高优先级。
2. **系统环境变量** (`GOMATE_HOST` / `GOMATE_PORT`)：次高优先级。
3. **默认值** (`localhost` / `52698`)：最低优先级。

## 应用场景

在远程 Windows VPS 上面执行安装脚本，通过 `gomate file` 命令发送到本地机器上面的 `Sublime Text`或者`VSCode` 等编辑器进行编辑。

| 角色                      | 运行地点     | 角色              | 端口       |
| ------------------------- | ------------ | ----------------- | ---------- |
| **编辑器 (Sublime Text)** | **本地机器** | **服务器 (监听)** | 监听 52698 |
| **客户端 (Gomate.exe)**   | **远程 VPS** | **客户端 (连接)** | 连接 52698 |

**❓问题：** Gomate 客户端在远程 Windows VPS 上，因为本地机器可能在防火墙或 NAT 后面，远程 Windows VPS上面的Gomate 客户端无法直接连接到本地机器的`52698` 端口 ，如何设置？

**✅方案：** 需要通过 **反向端口转发 (Reverse Port Forwarding)** 来实现。创建一个隧道，将 **本地 PC** 上的 `52698` 端口映射到 **VPS** 上的某个端口，使得 VPS 上的 `gomate.exe` 可以连接到这个 VPS 端口。 也就是`远程端口转发 (Remote Port Forwarding)`

### SSH 命令行参数 (`-R`)

需要使用 SSH 的 **远程端口转发 (`-R`)** 模式，在您从本地 PC SSH 连接到 VPS 时建立隧道。

#### 1. SSH 命令行参数 (`-R`)

在您的 **本地 PC** 上执行 SSH 连接到 VPS 的命令时，添加 `-R` 参数：

```Bash
ssh -R R_PORT:L_HOST:L_PORT user@VPS_HOST
```

| 参数         | 值          | 说明                                                     |
| ------------ | ----------- | -------------------------------------------------------- |
| **`-R`**     | (必须)      | 启用远程端口转发。                                       |
| **`R_PORT`** | 52698       | **VPS** 上监听的端口。`gomate.exe` 将连接此端口。        |
| **`L_HOST`** | `localhost` | **本地 PC** 上的目标地址（即 Sublime Text 监听的地址）。 |
| **`L_PORT`** | 52698       | **本地 PC** 上 Sublime Text 实际监听的端口。             |

示例 - 在**本地 PC**上运行：

```Bash
# 将本地 PC 的 52698 端口，转发到 VPS 的 52698 端口
ssh -R 52698:localhost:52698 user@your_vps_ip
```

#### 2. 操作流程

1. **本地 PC：** 启动 Sublime Text 并确保其监听 `localhost:52698`。
2. **本地 PC：** 执行上述带有 `-R` 参数的 SSH 命令，连接到 VPS。
3. **VPS：** 保持 SSH 会话打开（SSH 隧道正在运行）。
4. **VPS：** 执行 `gomate.exe`，并确保它连接到**它自己的** `localhost:52698`。

```Bash
# 在 VPS 上的 SSH 会话中运行
gomate.exe file_on_vps_to_edit.txt 
```



### 图形化工具配置 (Windows)

#### A. PuTTY

1. **Session（会话）**：输入 `Host Name` 和 `Port`。
2. **Connection > SSH > Tunnels（隧道）**：
   - **Source port (源端口)**: 输入 **VPS** 上的监听端口 (`52698`)。
   - **Destination (目标)**: 输入 **本地 PC** 的地址和端口 (`localhost:52698`)。
   - 选择 **Remote (远程)**。
   - 点击 **Add (添加)**。
3. 连接会话。

#### B. SecureCRT

1. **Session Options（会话选项）**：
2. 导航到 **Port Forwarding (端口转发) > Remote (远程)**。
3. 点击 **Add (添加)**。
4. **Name (名称)**: `Gomate_Reverse`
5. **Listen Port (监听端口)**: 输入 **VPS** 上的监听端口 (`52698`)。
6. **Remote (远程)**:
   - **Hostname**: `localhost`
   - **Port**: 输入 **本地 PC** 上的监听端口 (`52698`)。
7. 连接会话。

#### C. Termius

1. 在主机配置中，导航到 **Port forwarding (端口转发)**。
2. 点击 **+ New port forwarding**。
3. **Type (类型)**: 选择 **Remote (远程)**。
4. **Remote port (远程端口)**: 输入 VPS 上的端口 (`52698`)。
5. **Destination (目标)**: 输入本地 PC 的地址和端口 (`localhost:52698`)。
6. 保存并连接。



### 潜在的额外配置 (VPS)

某些 VPS 默认会限制远程转发的监听地址，可能导致 `-R` 转发的端口只能被 `localhost` 访问。

如果 `gomate.exe` 连接失败，您可能需要在 SSH 客户端或 VPS 上的 `sshd_config` 中设置：

**`sshd_config` 文件中 (VPS 端)**：添加 `GatewayPorts yes` 并重启 SSH 服务。

### **连接路径**

`gomate.exe` → `VPS:52698` → SSH 隧道 → `本地 PC:52698` → Sublime Text。

无论您使用哪种方法进行端口转发，一旦 SSH 隧道建立成功，您的 Gomate 客户端连接的地址始终是 本地回环地址：

```bash
# Gomate 默认连接：
gomate.cmd your_file.txt  # 默认连接 localhost:52698
```

如果您转发到了非默认端口，请通过命令行或环境变量指定：

```bash
# 如果 SSH 转发的是本地端口 60000 到远程 52698
gomate.cmd -port 60000 your_file.txt
```

### 在编辑器中启用监听

Gomate 客户端通过 TCP 连接发送文件数据和命令。您需要在本地机器上运行您选择的编辑器，并配置一个插件来监听网络连接。

#### 1. Sublime Text 3/4 

Sublime Text 是 Gomate 客户端协议的原型目标。您需要安装并配置 `remote_subl` 或兼容的插件。

##### A. 安装 `remote_subl` 插件

1. 打开 Sublime Text。
2. 打开 **Package Control** (`Ctrl+Shift+P` 或 `Cmd+Shift+P`)。
3. 选择 `Package Control: Install Package`。
4. 搜索并安装 **`remote_subl`**。

##### B. 配置监听端口

`remote_subl` 插件默认监听 `127.0.0.1:52698`。如果需要修改端口或监听所有网络接口，请修改插件设置：

1. 打开 **Preferences > Package Settings > remote_subl > Settings - User**。
2. 添加或修改以下配置：

```JSON
{
    // 确保监听所有网络接口，以便 SSH 隧道可以连接到它
    "host": "127.0.0.1", 
    // 确保端口与 Gomate 配置一致（默认 52698）
    "port": 52698 
}
```

##### C. 启动监听

`remote_subl` 通常在 Sublime Text 启动时自动开始监听。如果没有，请检查插件的输出日志。

#### Visual Studio Code (VS Code)

使用 **`remote-vscode`** 插件可以使 VS Code 兼容 Gomate 客户端所使用的 `remote_subl` 协议，从而在 VS Code 中进行远程文件编辑。

##### A. 安装 `remote-vscode` 插件

1. 打开 VS Code。
2. 进入 **Extensions (扩展)** 视图 (`Ctrl+Shift+X` 或 `Cmd+Shift+X`)。
3. 搜索 `rafaelmaiolla.remote-vscode` 或直接搜索 **`remote-vscode`**。
4. 点击安装。

##### B. 端口配置 (默认与 Gomate 兼容)

该插件默认监听端口 `52698`，与 Gomate 客户端的默认配置完全一致。

如果您需要修改端口，请进行如下操作：

1. 打开 **Settings (设置)** (`Ctrl+,` 或 `Cmd+,`)。
2. 搜索 `remote-vscode port`。
3. 找到 `Remote Vscode: Port` 选项，将其值修改为您需要的端口（例如 `52699`）。

> **重要提示：** 如果您修改了此端口，请确保您的 Gomate 客户端（通过 `-port` 参数或 `GOMATE_PORT` 环境变量）或 SSH 隧道配置与其保持一致。

##### C. 启动监听

`remote-vscode` 插件通常在 VS Code 启动时自动开始监听配置的端口。

当您在远程 VPS 上执行 `gomate.exe` 时，它将通过 SSH 隧道连接到您本地 VS Code 正在监听的端口，并在 VS Code 中打开文件。



由于 Gomate 客户端是基于 `remote_subl` 协议构建的，**Sublime Text 仍是实现完整功能和最佳兼容性的首选编辑器。**

请确保：

1. 编辑器正在运行。
2. 相应的插件已安装并激活。
3. 插件监听的端口与您在 Gomate 客户端或 SSH 隧道中配置的端口**完全一致**。


