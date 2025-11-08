@echo off

:: --------------------------------------------------------------------------------
:: Gomate Windows 启动脚本 (Version 1.0)
:: --------------------------------------------------------------------------------

:: 必须开启，用于在循环中追踪变量
setlocal enabledelayedexpansion

set "DEBUG_ECHO=if "%%VERBOSE_MODE%%" equ "1" echo"

:: %~dp0 gomate.cmd 所在文件夹，比如 C:\SW\gomate\
:: C:\SW\gomate\gomate.cmd  
:: C:\SW\gomate\core\gomate.exe
:: C:\SW\gomate\core\gomate.vbs

set "GOMATE_EXE_PATH=%~dp0core\gomate.exe"
set "GOMATE_VBS_PATH=%~dp0core\gomate.vbs"

if not exist "%GOMATE_EXE_PATH%" (
    echo ERROR: gomate.exe not found! Expected path: "%GOMATE_EXE_PATH%"
    goto :eof
)
if not exist "%GOMATE_VBS_PATH%" (
    echo ERROR: gomate.vbs not found! Expected path: "%GOMATE_VBS_PATH%"
    goto :eof
)

if "%1"=="" (
    echo.
    echo Usage: gomate.cmd [OPTIONS] file_path [file_path ...]
    echo   -v, --verbose    Verbose logging messages.
    echo   -w, --wait       Wait for file to be closed by editor.
    echo   -f, --force      Open even if file is not writable.
    echo   -n, --new        Open in a new window Sublime Text.
    echo   -h, --host HOST  Connect to HOST. Defaults to 'localhost'.
    echo   -p, --port PORT  Port number to use for connection. Defaults to 52698.
    echo   -m, --name NAME  The display name shown in editor.
    echo   -t, --type TYPE  Treat file as having specified type.
    echo   -l, --line LINE  Place caret on line number after loading file.
    goto :eof
)

:: --------------------------------------------------------------------------------
:: A. 初始化变量
:: --------------------------------------------------------------------------------
set "ALL_ARGS=%*"             :: 完整的原始参数列表 (用于传递给 EXE)
set "WAIT_MODE=0"             :: 默认不等待 (0)
set "VERBOSE_MODE=0"           :: 默认隐藏 (0)
set "FILE_COUNT=0"            :: 文件计数器
set "LAST_WAS_FLAG=0"         :: 追踪前一个参数是否为 Flag (0=否, 1=是)

:: --------------------------------------------------------------------------------
:: B. 遍历所有参数
:: --------------------------------------------------------------------------------
:CheckArgsLoop

if "%1"=="" goto :EndChecks  

set "CURRENT_ARG=%~1"
set "IS_FLAG=0"

:: 检查是否为 Flag
if "!CURRENT_ARG:~0,1!" == "-" set "IS_FLAG=1"
if "!CURRENT_ARG:~0,1!" == "/" set "IS_FLAG=1"

:: --------------------------------------------------------------------------------
:: 1. 检查 Flag
:: --------------------------------------------------------------------------------

if "!IS_FLAG!" equ "1" (
    :: 当前参数是 Switch
    if /i "%~1" equ "-v"       set "VERBOSE_MODE=1"        & goto :SkipValueFlagCheck
    if /i "%~1" equ "-verbose" set "VERBOSE_MODE=1"        & goto :SkipValueFlagCheck
  
    if /i "%~1" equ "-w"       set "WAIT_MODE=1"           & goto :SkipValueFlagCheck
    if /i "%~1" equ "-wait"    set "WAIT_MODE=1"           & goto :SkipValueFlagCheck

    if /i "%~1" equ "-f"       goto :SkipValueFlagCheck
    if /i "%~1" equ "-force"   goto :SkipValueFlagCheck

    if /i "%~1" equ "-n"       goto :SkipValueFlagCheck
    if /i "%~1" equ "-new"     goto :SkipValueFlagCheck

    :: 当前参数不是 Switch
    if /i "%~1" equ "-h"       set "LAST_WAS_VALUE_FLAG=1" & goto :SkipCount
    if /i "%~1" equ "-host"    set "LAST_WAS_VALUE_FLAG=1" & goto :SkipCount

    if /i "%~1" equ "-p"       set "LAST_WAS_VALUE_FLAG=1" & goto :SkipCount
    if /i "%~1" equ "-port"    set "LAST_WAS_VALUE_FLAG=1" & goto :SkipCount

    if /i "%~1" equ "-m"       set "LAST_WAS_VALUE_FLAG=1" & goto :SkipCount
    if /i "%~1" equ "-name"    set "LAST_WAS_VALUE_FLAG=1" & goto :SkipCount

    if /i "%~1" equ "-l"       set "LAST_WAS_VALUE_FLAG=1" & goto :SkipCount
    if /i "%~1" equ "-line"    set "LAST_WAS_VALUE_FLAG=1" & goto :SkipCount

    if /i "%~1" equ "-t"       set "LAST_WAS_VALUE_FLAG=1" & goto :SkipCount
    if /i "%~1" equ "-type"    set "LAST_WAS_VALUE_FLAG=1" & goto :SkipCount
    
    :SkipValueFlagCheck

    :: 开关 Flag 不影响下一个参数
    set "LAST_WAS_VALUE_FLAG=0"

    goto :SkipCount
)

:: --------------------------------------------------------------------------------
:: 2. 文件计数逻辑
:: --------------------------------------------------------------------------------

if "!LAST_WAS_VALUE_FLAG!" equ "1" (
    :: 当前是带值参数, 处理完值后重置
    set "LAST_WAS_VALUE_FLAG=0"
    goto :SkipCount
)

:: 如果到达这里，它就是文件。
set /a FILE_COUNT+=1
set "LAST_WAS_VALUE_FLAG=0"


:: 检查下一个参数
:SkipCount
shift
goto :CheckArgsLoop
  
:: 检查参数完成
:EndChecks

:: --------------------------------------------------------------------------------
:: C. 最终检查文件数量
:: --------------------------------------------------------------------------------
if !FILE_COUNT! equ 0 (
    echo ERROR: No file path provided!
    echo.
    echo Usage: gomate.cmd [OPTIONS] file_path [file_path ...]
    echo.
    goto :eof
)

:: --------------------------------------------------------------------------------
:: D. 构造命令字符串
:: --------------------------------------------------------------------------------
:: CMD_LINE = "C:\path\to\gomate.exe" arg1 arg2 ... (使用原始参数列表)
:: set "CMD_LINE="%GOMATE_EXE_PATH%" %ALL_ARGS%"
:: 只将 ALL_ARGS 赋值给一个变量，不包含 EXE 路径，避免引号嵌套问题。
set "GOMATE_ARGS=%ALL_ARGS%"


:: 调试输出现在显示的是实际参数
!DEBUG_ECHO! [Gomate] EXE  Path: "%GOMATE_EXE_PATH%"
!DEBUG_ECHO! [Gomate] VBS  Path: "%GOMATE_VBS_PATH%"
!DEBUG_ECHO! [Gomate] Arguments: !GOMATE_ARGS!
!DEBUG_ECHO! [Gomate] Wait Mode: %WAIT_MODE%
!DEBUG_ECHO! [Gomate]   Versose: %VERBOSE_MODE%

:: --------------------------------------------------------------------------------
:: E. 异步模式执行
:: --------------------------------------------------------------------------------

if "!WAIT_MODE!" equ "1" goto :ExecuteWaitMode

if "!VERBOSE_MODE!" equ "1" (
  set "VBS_WINDOW_ARG=1"
  echo [Gomate] Starting editing process - Show Window.
) else (
  set "VBS_WINDOW_ARG=0"
  echo [Gomate] Starting editing process - Hide Window.
)

:: 构造 VBScript CMD LINE
set "VBS_CMD_LINE="%GOMATE_EXE_PATH%" !GOMATE_ARGS!"

:: 异步模式执行
cscript.exe //Nologo //B "%GOMATE_VBS_PATH%" "!VBS_CMD_LINE!" "!VBS_WINDOW_ARG!"
goto :EndExecution

:: --------------------------------------------------------------------------------
:: F. 同步模式执行
:: --------------------------------------------------------------------------------
:ExecuteWaitMode

echo [Gomate] Starting editing process - Current Window
echo [Gomate] The current window will block until the editor is closed...

"%GOMATE_EXE_PATH%" !GOMATE_ARGS!

:: --------------------------------------------------------------------------------
:EndExecution
:eof
endlocal
