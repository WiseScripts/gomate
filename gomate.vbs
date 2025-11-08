' gomate.vbs
Dim WshShell
Set WshShell = CreateObject("WScript.Shell")

Dim cmdLine
cmdLine = WScript.Arguments(0) ' cmdLine = "C:\path\to\gomate.exe" -v file.txt

Dim windowMode
windowMode = WScript.Arguments(1) ' "0" (Hidden) or "1" (Visible)

' intWindowStyle: 10 = SW_SHOWDEFAULT (新窗口), 0 = SW_HIDE (隐藏)
If windowMode = "1" Then
    ' 启动新窗口 (新进程)
    WshShell.Run cmdLine, 10, False
Else
    ' 隐藏启动
    WshShell.Run cmdLine, 0, False
End If

Set WshShell = Nothing
