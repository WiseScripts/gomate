# --------------------------------------------------------------------------------------
# 文件名: install-gomate.ps1 (一键安装脚本 - 路径可配置)
# 用法: irm https://raw.githubusercontent.com/.../install-gomate.ps1 | iex
# --------------------------------------------------------------------------------------

param(
    [Parameter(Mandatory=$false)]
    [string]$hostname = "localhost",

    [Parameter(Mandatory=$false)]
    [int]$port = 52698,

    [Parameter(Mandatory=$false)]
    [string]$InstallDir = "C:\SW\gomate" # <--- 默认安装路径
)

# --- 配置信息 (请根据您的仓库信息修改) ---
$GitHubUser = "WiseScripts" # <-- 替换为您的 GitHub 用户名
$GitHubRepo = "gomate" # <-- 替换为您的仓库名
$AssetName = "gomate.exe"

# --- 动态路径定义 ---
$gomateDir = $InstallDir
$gomateCoreDir = Join-Path -Path $gomateDir -ChildPath "core"

$gomateCmdPath = Join-Path -Path $gomateDir -ChildPath "gomate.cmd"
$gomateVbsPath = Join-Path -Path $gomateCoreDir -ChildPath "gomate.vbs"
$gomateExePath = Join-Path -Path $gomateCoreDir -ChildPath $AssetName

# --- 仓库文件下载 URL ---
$GitHubBaseUrl = "https://raw.githubusercontent.com/$GitHubUser/$GitHubRepo/refs/heads/main"
$CmdUrl = "$GitHubBaseUrl/gomate.cmd"
$VbsUrl = "$GitHubBaseUrl/gomate.vbs"


# --- 辅助函数：设置系统环境变量 (保持不变) ---
function Set-SystemEnvironmentVariable {
    param([Parameter(Mandatory=$true)][string]$Name, [Parameter(Mandatory=$true)][string]$Value)
    Write-Host "✅ System environment variable $Name set to: $Value" -ForegroundColor Green
    [Environment]::SetEnvironmentVariable($Name, $Value, [EnvironmentVariableTarget]::Machine)
}

# --------------------------------------------------------------------------------------
# --- 1. 下载最新 Gomate.exe ---
# --------------------------------------------------------------------------------------
Write-Host "--- 1. Downloading latest $AssetName ---" -ForegroundColor Yellow

try {
    # 查找最新 Release 的 API URL
    $ApiUrl = "https://api.github.com/repos/$GitHubUser/$GitHubRepo/releases/latest"
    $ReleaseInfo = Invoke-RestMethod -Uri $ApiUrl

    # 查找 gomate.exe 附件的下载 URL
    $Asset = $ReleaseInfo.assets | Where-Object { $_.name -eq $AssetName }
    if (-not $Asset) { throw "Could not find asset '$AssetName' in the latest release." }
    
    $DownloadUrl = $Asset.url
    
    # 确保核心目录存在
    if (-not (Test-Path $gomateCoreDir)) { New-Item -Path $gomateCoreDir -ItemType Directory -Force | Out-Null }
    
    # 下载文件 (需要设置 Headers 来处理重定向和认证)
    Invoke-RestMethod -Uri $DownloadUrl -Headers @{"Accept"="application/octet-stream"} -OutFile $gomateExePath
    Write-Host "✅ Downloaded $AssetName to $gomateExePath" -ForegroundColor Green

} catch {
    Write-Error "Download failed: $($_.Exception.Message)"
    return
}


# --------------------------------------------------------------------------------------
# --- 2. 核心配置逻辑 (从 GitHub 下载 .cmd 和 .vbs) ---
# --------------------------------------------------------------------------------------

Write-Host "`n--- 2. Setting up configuration scripts and PATH ---" -ForegroundColor Yellow

# 确保目录存在
if (-not (Test-Path $gomateDir)) { New-Item -Path $gomateDir -ItemType Directory -Force | Out-Null }

# 确保核心目录也存在
if (-not (Test-Path $gomateCoreDir)) { New-Item -Path $gomateCoreDir -ItemType Directory -Force | Out-Null } 

try {
    # --- 2.1 下载 gomate.exe 隐藏启动器 (gomate.vbs) ---
    Write-Host "Downloading gomate.vbs from repository: $VbsUrl"
    # 使用 Invoke-WebRequest 下载原始文件内容
    Invoke-WebRequest -Uri $VbsUrl -OutFile $gomateVbsPath -UseBasicParsing -ErrorAction Stop
    Write-Host "✅ Downloaded gomate.vbs to $gomateVbsPath" -ForegroundColor Green

    # --- 2.2 下载 gomate.exe 启动脚本 (gomate.cmd) ---
    Write-Host "Downloading gomate.cmd from repository: $CmdUrl"
    # 使用 Invoke-WebRequest 下载原始文件内容
    Invoke-WebRequest -Uri $CmdUrl -OutFile $gomateCmdPath -UseBasicParsing -ErrorAction Stop
    Write-Host "✅ Downloaded gomate.cmd to $gomateCmdPath" -ForegroundColor Green
    
} catch {
    Write-Error "Failed to download configuration scripts: $($_.Exception.Message)"
    return
}


# --- 3. 设置系统环境变量 ---
Write-Host "`n--- 3. Setting System Environment Variables (GOMATE_HOST/GOMATE_PORT) ---" -ForegroundColor Yellow

if (-not [string]::IsNullOrEmpty($hostname)) {
    Set-SystemEnvironmentVariable -Name "GOMATE_HOST" -Value $hostname
} else {
    Write-Host "Skipping GOMATE_HOST setting. Use --host or set env var manually." -ForegroundColor Cyan
}

if ($port -ne 0) {
    Set-SystemEnvironmentVariable -Name "GOMATE_PORT" -Value ([string]$port)
} else {
    Write-Host "Skipping GOMATE_PORT setting. Use --port or set env var manually." -ForegroundColor Cyan
}


# --- 4. 更新系统 PATH (PATH 优先级修复) ---
Write-Host "`n--- 4. Updating System PATH Environment Variable ---" -ForegroundColor Yellow

$currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")

# 确保 $gomateDir (包含 .cmd) 位于 PATH 最前端
$pathToAdd = $gomateDir
$newPath = $currentPath

if ($currentPath -notlike "*$pathToAdd*") {
    $newPath = "$pathToAdd;$newPath"
}

# 确保 $gomateCoreDir 也在 PATH 中
if ($newPath -notlike "*$gomateCoreDir*") {
    # $newPath = "$gomateCoreDir;$newPath"
}


[Environment]::SetEnvironmentVariable("Path", $newPath, [EnvironmentVariableTarget]::Machine)

Write-Host "✅ Path updated. $gomateDir is now prioritized." -ForegroundColor Green
Write-Host "`n--- Installation Complete ---" -ForegroundColor Green
Write-Host "❗ Please restart your terminal or SSH session to load the new PATH." -ForegroundColor Yellow
