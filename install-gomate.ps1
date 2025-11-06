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
$GitHubUser = "WiseScripts"  # <-- 替换为您的 GitHub 用户名
$GitHubRepo = "gomate" # <-- 替换为您的仓库名
$AssetName = "gomate.exe"


# --- 动态路径定义 ---
$gomateDir = $InstallDir
$gomateCoreDir = Join-Path -Path $gomateDir -ChildPath "core"

$gomateCmdPath = Join-Path -Path $gomateDir -ChildPath "gomate.cmd"
$gomateExePath = Join-Path -Path $gomateCoreDir -ChildPath $AssetName
$gomateVbsPath = Join-Path -Path $gomateCoreDir -ChildPath "gomate.vbs"


# --- 辅助函数：设置系统环境变量 (保持不变) ---
function Set-SystemEnvironmentVariable {
    param([Parameter(Mandatory=$true)][string]$Name, [Parameter(Mandatory=$true)][string]$Value)
    [Environment]::SetEnvironmentVariable($Name, $Value, [EnvironmentVariableTarget]::Machine)
    Write-Host "✅ System environment variable $Name set to: $Value" -ForegroundColor Green
}

# --------------------------------------------------------------------------------------
# --- 1. 下载最新 Gomate.exe ---
# --------------------------------------------------------------------------------------
Write-Host "--- 1. Downloading latest $AssetName ---" -ForegroundColor Yellow

# ... (下载逻辑与之前保持一致，确保下载到 $gomateExePath) ...
# (为简洁省略下载 API 调用代码，请保留原 download 逻辑)

try {
    # 查找最新 Release 的 API URL
    $ApiUrl = "https://api.github.com/repos/$GitHubUser/$GitHubRepo/releases/latest"
    $ReleaseInfo = Invoke-RestMethod -Uri $ApiUrl

    # 查找 gomate.exe 附件的下载 URL
    $Asset = $ReleaseInfo.assets | Where-Object { $_.name -eq $AssetName }
    if (-not $Asset) { throw "Could not find asset '$AssetName' in the latest release." }
    
    $DownloadUrl = $Asset.url
    
    # 确保目录存在
    if (-not (Test-Path $gomateCoreDir)) { New-Item -Path $gomateCoreDir -ItemType Directory -Force | Out-Null }
    
    # 下载文件 (需要设置 Headers 来处理重定向和认证)
    Invoke-RestMethod -Uri $DownloadUrl -Headers @{"Accept"="application/octet-stream"} -OutFile $gomateExePath
    Write-Host "✅ Downloaded $AssetName to $gomateExePath" -ForegroundColor Green

} catch {
    Write-Error "Download failed: $($_.Exception.Message)"
    return
}


# --------------------------------------------------------------------------------------
# --- 2. 核心配置逻辑 ---
# --------------------------------------------------------------------------------------

Write-Host "`n--- 2. Setting up configuration scripts and PATH ---" -ForegroundColor Yellow

if (-not (Test-Path $gomateDir)) { New-Item -Path $gomateDir -ItemType Directory -Force | Out-Null }
if (-not (Test-Path $gomateCoreDir)) { New-Item -Path $gomateCoreDir -ItemType Directory -Force | Out-Null } # 确保核心目录也存在

# --- 2.1 创建通用 VBScript 隐藏启动器 (gomate.vbs) ---
# VBScript 内容：接收一个命令字符串，并使用 Shell.Run(cmd, 0, false) 隐藏执行。
$vbsContent = @"
Set WshShell = CreateObject("WScript.Shell")
command = Chr(34) & WScript.Arguments(0) & Chr(34) & " " & WScript.Arguments(1)
WshShell.Run command, 0, False
"@

$vbsContent | Out-File -FilePath $gomateVbsPath -Encoding ASCII
Write-Host "✅ Hidden VBScript launcher $gomateVbsPath created." -ForegroundColor Green

# --- 2.2 创建 gomate.cmd (直接启动 VBScript) ---
$cmdContent = @"
@echo off
setlocal
set "GOMATE_EXE_PATH=%~dp0core\gomate.exe"
set "GOMATE_VBS_PATH=%~dp0core\gomate.vbs"

if not exist "%GOMATE_EXE_PATH%" (
    echo.
    echo ERROR: gomate.exe not found! Deploy to %GOMATE_EXE_PATH%
    goto :eof
)

if "%1"=="" (
    echo.
    echo ERROR: Please provide the file path to edit.
    goto :eof
)

set "ALL_ARGS=%*"
cscript.exe //Nologo //B "%GOMATE_VBS_PATH%" "%GOMATE_EXE_PATH%" "%ALL_ARGS%"

echo.
echo [Gomate] Starting background editing process (Hidden, VBScript)...
echo.

endlocal
"@
$cmdContent | Out-File -FilePath $gomateCmdPath -Encoding ASCII
Write-Host "✅ Unified launcher $gomateCmdPath created." -ForegroundColor Green


# --- 3. 设置系统环境变量 ---
Write-Host "`n--- 3. Setting System Environment Variables (GOMATE_HOST/PORT) ---" -ForegroundColor Yellow

if (-not [string]::IsNullOrEmpty($hostname)) {
    Set-SystemEnvironmentVariable -Name "GOMATE_HOST" -Value $hostname
} else {
    Write-Host "Skipping GOMATE_HOST setting. Use --hostname or set env var manually." -ForegroundColor Cyan
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
    $newPath = "$gomateCoreDir;$newPath"
}


[Environment]::SetEnvironmentVariable("Path", $newPath, [EnvironmentVariableTarget]::Machine)

Write-Host "✅ Path updated. $gomateDir is now prioritized." -ForegroundColor Green
Write-Host "`n--- Installation Complete ---" -ForegroundColor Green
Write-Host "❗ Please restart your terminal or SSH session to load the new PATH." -ForegroundColor Yellow
