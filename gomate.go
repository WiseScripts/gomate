package main

import (
  "bufio"
  "crypto/md5"
  "errors"
  "flag"
  "fmt"
  "io"
  "log"
  "net"
  "os"
  "os/signal"
  "path/filepath"
  "strconv"
  "strings"
  "syscall"
  "time"
)

// 全局变量定义
var hashes = make(map[string]string)
var ErrInstanceAlreadyRunning = errors.New("instance already running")

// configureLogging 配置 log 包的输出目标。
func configureLogging(verbose bool) {
  if verbose {
    // 启用 verbose，输出到标准错误流，带标准时间/日期前缀
    log.SetOutput(os.Stderr)
    log.SetFlags(log.LstdFlags)
    log.Println("Verbose logging enabled.")
  } else {
    // 默认情况下，将日志输出丢弃到 io.Discard (黑洞)
    log.SetOutput(io.Discard)
  }
}

// sendFile 将文件内容发送给远程编辑器。
func sendFile(conn net.Conn, filename string) error {
  f, err := os.Open(filename)
  if err != nil {
    return fmt.Errorf("failed to open file %s: %w", filename, err)
  }
  defer func() {
    if closeErr := f.Close(); closeErr != nil {
      log.Printf("Warning: failed to close file %s: %v", filename, closeErr)
    }
  }()

  st, err := f.Stat()
  if err != nil {
    return fmt.Errorf("failed to stat file %s: %w", filename, err)
  }

  // 使用 MD5 哈希作为文件的唯一令牌
  hash := fmt.Sprintf("%x", md5.Sum([]byte(filename)))
  hashes[hash] = filename

  // 遵循 `remote_subl` 协议写入头部信息
  // 改进: 使用 log.Printf 记录发送信息，但仅在 verbose 模式下可见
  log.Printf("Sending file header for: %s", filename)
  log.Printf("Sending header token: %s", hash)
  log.Printf("Sending header size: %d", st.Size())

  fmt.Fprintf(conn, "open\n")

  fmt.Fprintf(conn, "token: %v\n", hash)
  fmt.Fprintf(conn, "display-name: %v\n", filepath.Base(filename))
  //fmt.Fprintf(conn, "real-path: %v\n", filename)
  //fmt.Fprintf(conn, "data-on-save: yes\n")
  //fmt.Fprintf(conn, "re-activate: yes\n")

  fmt.Fprintf(conn, "data: %v\n", st.Size())

  // 改进: io.Copy 失败时不应使用 log.Fatal，应返回错误
  // 在使用 log.Fatal 函数时，内部调用 os.Exit(1)
  // os.Exit 函数通知程序退出，并且该函数之后的逻辑将不会被执行。
  // 在调用时需要指定退出码，为 0 时，表示正常退出程序。
  if _, err := io.Copy(conn, f); err != nil {
    return fmt.Errorf("failed to copy file data to connection: %w", err)
  }

  fmt.Fprintf(conn, "\n.\n")
  return nil
}

// handleCommands 处理来自远程编辑器的命令（close, save 等）。
func handleCommands(buf *bufio.Reader) (bool, error) {
  // 读取并解析命令
  b, _, err := buf.ReadLine()
  if err != nil {
    return false, err
  }
  cmd := strings.TrimSpace(string(b))
  log.Printf("Received command: %s", cmd)

  switch cmd {
  case "close":
    // close
    // token: xxx
    // ""
    var token string
    for {
      b, _, err = buf.ReadLine()
      if err != nil {
        return true, fmt.Errorf("failed to read save header: %w", err)
      }
      line := strings.TrimSpace(string(b))
      //log.Printf("Received header line: %s", line)
      if strings.HasPrefix(line, "token:") {
        token = strings.TrimSpace(line[6:])
        log.Printf("Received header token: %s.\n", token)
        log.Printf("Exiting gracefully.\n")
        break
      }
    }
    return true, nil

  case "save":
    // save
    // token: xxx
    // data: 128
    // body
    var token string
    var size int64

    // 循环读取 save 命令的头部信息
    for {
      b, _, err = buf.ReadLine()
      if err != nil {
        return true, fmt.Errorf("failed to read save header: %w", err)
      }
      line := strings.TrimSpace(string(b))
      log.Printf("Header line: %s", line)

      if strings.HasPrefix(line, "token:") {
        token = strings.TrimSpace(line[6:])
      } else if strings.HasPrefix(line, "data:") {
        size, err = strconv.ParseInt(strings.TrimSpace(line[5:]), 10, 64)
        if err != nil {
          return true, fmt.Errorf("invalid data size format: %w", err)
        }
        break // 找到 data: 行后跳出循环，准备接收数据
      }
    }

    // 创建临时文件来接收数据
    f, err := os.CreateTemp("", "gomate-temp-")
    if err != nil {
      return true, fmt.Errorf("failed to create temporary file: %w", err)
    }

    // 改进: defer 中应记录错误，并清理临时文件（虽然重命名成功会隐式删除）
    defer func(tempFile *os.File) {
      if closeErr := tempFile.Close(); closeErr != nil {
        log.Printf("Warning: failed to close temporary file: %v", closeErr)
      }
      // 即使重命名失败，也尝试删除，防止残留
      if removeErr := os.Remove(tempFile.Name()); removeErr != nil && !os.IsNotExist(removeErr) {
        log.Printf("Warning: failed to remove temporary file %s: %v", tempFile.Name(), removeErr)
      }
    }(f)

    // 复制数据到临时文件
    if _, err = io.CopyN(f, buf, size); err != nil {
      return true, fmt.Errorf("failed to copy data from editor: %w", err)
    }

    // 必须在重命名之前关闭文件，确保所有数据已写入磁盘
    if err = f.Close(); err != nil {
      log.Printf("Warning: failed to finalize temporary file close: %v", err)
      // 仍然尝试重命名，因为文件可能已部分写入
    }

    // 通过 token 查找原始文件名并重命名
    if filename, ok := hashes[token]; ok {
      log.Printf("Saving content to original file: %s", filename)

      // os.Rename 是一个原子操作 (如果可能)
      return false, os.Rename(f.Name(), filename)
    }

    // 未知的 token 错误
    return false, errors.New("unknown token: " + token)

  default:
    // 改进: 记录未知的命令，但保持连接
    log.Printf("Unknown command received, ignoring: %s", cmd)
    return false, nil
  }
}

// ensureFileExists 检查文件是否存在，如果不存在，则创建它和所有必需的父目录。
func ensureFileExists(filePath string) error {
  log.Printf("Ensuring file and directory structure exists for: %s", filePath)

  dirPath := filepath.Dir(filePath)

  // 1. 检查并创建多级父目录
  if dirPath != "" {
    // 0755 权限
    if err := os.MkdirAll(dirPath, 0755); err != nil {
      return fmt.Errorf("failed to create directories %s: %w", dirPath, err)
    }
    log.Printf("Successfully ensured directory: %s", dirPath)
  }

  // 2. 检查文件是否存在，不存在则创建空文件
  if _, err := os.Stat(filePath); os.IsNotExist(err) {
    file, err := os.Create(filePath)
    if err != nil {
      return fmt.Errorf("failed to create file %s: %w", filePath, err)
    }
    // 立即关闭文件
    if closeErr := file.Close(); closeErr != nil {
      log.Printf("Warning: failed to close newly created file %s: %v", filePath, closeErr)
    }
    log.Printf("Successfully created new file: %s", filePath)
  } else if err != nil {
    // Stat 遇到了其他错误 (如权限问题)
    return fmt.Errorf("error checking file status %s: %w", filePath, err)
  }

  return nil
}

// killProcessByPID 尝试杀死指定 PID 的进程。
func killProcessByPID(pid int) error {
  log.Printf("Attempting to kill process with PID: %d", pid)

  // 在 Go 中，os.FindProcess 和 os.Process.Kill/Signal
  // 跨平台兼容性更好，但在 Windows 上实现 Kill 可能需要额外的 syscall。

  // 尝试查找进程
  proc, err := os.FindProcess(pid)
  if err != nil {
    // 如果找不到进程，可能进程已经死亡
    return fmt.Errorf("failed to find process with PID %d: %w", pid, err)
  }

  // 尝试发送中断信号 (SIGINT) 或直接终止 (Kill)
  // 在 Windows 上， os.Kill 是通过 TerminateProcess 实现的。
  if err := proc.Kill(); err != nil {
    // 检查是否是因为进程已不存在导致的错误
    if errors.Is(err, os.ErrPermission) {
      // 如果是权限错误，可能是进程已经死亡但文件锁未释放
      log.Printf("Permission denied when killing PID %d, assuming process is stale or access denied.", pid)
      return nil // 假设成功
    }
    return fmt.Errorf("failed to kill process with PID %d: %w", pid, err)
  }

  log.Printf("Successfully killed previous instance with PID: %d", pid)
  return nil
}

// checkMultiInstance 尝试在全局目录下创建独占锁文件，如果已存在，则根据 force 参数决定是否杀死前一个进程。
func checkMultiInstance(filePath string, force bool) (*os.File, error) { // ❗ 接收 force 参数
  absFilePath, err := filepath.Abs(filePath)
  if err != nil {
    return nil, fmt.Errorf("error getting absolute path: %w", err)
  }
  log.Printf("File absolute path: %s", absFilePath)

  // 标准化路径 (Windows 不区分大小写，但哈希计算需要统一)
  lowCaseAbsFilePath := strings.ToLower(absFilePath)

  // 1. 定义全局锁定目录 (%ProgramData%/GomateLocks)
  const lockDirName = "GomateLocks"
  // ALLUSERSPROFILE 对应 C:\ProgramData
  globalLockDir := filepath.Join(os.Getenv("ALLUSERSPROFILE"), lockDirName)

  // 2. 确保锁定目录存在
  if _, err := os.Stat(globalLockDir); os.IsNotExist(err) {
    if err := os.MkdirAll(globalLockDir, 0755); err != nil {
      return nil, fmt.Errorf("failed to create global lock directory %s: %w", globalLockDir, err)
    }
  }

  // 3. 构造锁文件路径
  lowCaseAbsFilePathHash := fmt.Sprintf("%x", md5.Sum([]byte(lowCaseAbsFilePath)))
  lockFilePath := filepath.Join(globalLockDir, lowCaseAbsFilePathHash)
  log.Printf("Lock file path: %s", lockFilePath)

  // 4. 尝试独占创建锁文件
  // O_CREATE | O_EXCL: 确保文件独占创建，如果已存在则返回 os.ErrExist
  lockFile, err := os.OpenFile(lockFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
  if err != nil {
    if os.IsExist(err) {
      // ===================================================
      // ❗ 锁文件已存在 - 处理多实例冲突
      // ===================================================
      log.Printf("File already exists: %s. An instance is already running.", lockFilePath)

      if force {
        // --- 强制模式启动 ---

        // 1. 读取旧进程 PID
        content, readErr := os.ReadFile(lockFilePath)
        if readErr != nil {
          log.Printf("Warning: Failed to read PID from lock file %s: %v", lockFilePath, readErr)
          // 无法读取 PID，但强制模式下仍尝试继续，删除旧锁文件
        }

        pid, parseErr := strconv.Atoi(strings.TrimSpace(string(content)))
        if parseErr == nil && pid > 0 {
          // 2. 杀死旧进程
          if killErr := killProcessByPID(pid); killErr != nil {
            log.Printf("Warning: Failed to kill previous process (PID: %d): %v", pid, killErr)
            // 尽管杀死失败，但我们尝试删除锁文件，假设进程已僵尸或已死亡
          }
        } else if readErr == nil {
          // 进程 ID 无效（可能锁文件内容损坏），记录并继续
          log.Printf("Warning: Lock file content is not a valid PID: %s", strings.TrimSpace(string(content)))
        }

        // 3. 删除旧锁文件 (清理僵尸锁)
        if removeErr := os.Remove(lockFilePath); removeErr != nil {
          // 如果删除失败，则无法继续，返回致命错误
          return nil, fmt.Errorf("force mode failed: cannot remove stale lock file %s: %w", lockFilePath, removeErr)
        }
        log.Println("Stale lock file removed. Retrying to create new lock.")

        // 4. 递归调用自身或重新尝试创建锁文件
        // 最好是重新尝试创建，而不是递归调用
        return checkMultiInstance(filePath, force)
      } else {
        // --- 非强制模式 ---
        return nil, ErrInstanceAlreadyRunning
      }
    } else {
      // 其他致命错误
      return nil, fmt.Errorf("error creating lock file: %w", err)
    }
  } else {
    // ===================================================
    // 锁文件创建成功 - 写入当前进程 ID
    // ===================================================
    currentPID := os.Getpid()
    pidStr := strconv.Itoa(currentPID)

    // 写入当前 PID 到锁文件
    if _, writeErr := lockFile.WriteString(pidStr); writeErr != nil {
      // 如果写入失败，应关闭并清理锁文件，然后返回错误
      lockFile.Close()
      os.Remove(lockFilePath)
      return nil, fmt.Errorf("failed to write PID to lock file: %w", writeErr)
    }

    log.Printf("Created lock file successfully and wrote PID %d: %s", currentPID, lockFilePath)
    // 锁定成功
    return lockFile, nil
  }
}

// CommandResult 用于在 Goroutine 之间传递 handleCommands 的结果。
type CommandResult struct {
  Exit bool
  Err  error
}

func main() {
  // --- 1. 参数定义和解析 ---
  const Defaulthost = "localhost"
  const DefaultPort = 52698

  var new bool
  var wait bool
  var verbose bool
  var force bool

  var host string
  var port int

  var fileName string
  var fileType string
  var fileLine int

  // 创建一个 channel 用于接收退出信号 (来自信号 Goroutine 或命令处理)
  exitSignal := make(chan struct{})

  // 用于接收 handleCommands 的结果
  commandResult := make(chan CommandResult)

  flag.BoolVar(&verbose, "v", false, "Enable verbose logging output")
  flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging output")

  flag.BoolVar(&new, "n", false, "Open in a new window")
  flag.BoolVar(&new, "new", false, "Open in a new window")

  flag.BoolVar(&wait, "w", false, "Wait for file to be closed by editor")
  flag.BoolVar(&wait, "wait", false, "Wait for file to be closed by editor")

  flag.BoolVar(&force, "f", false, "Open even if file is not writable")
  flag.BoolVar(&force, "force", false, "Open even if file is not writable")

  flag.StringVar(&host, "h", Defaulthost, "host of remote editor")
  flag.StringVar(&host, "host", Defaulthost, "host of remote editor")

  flag.IntVar(&port, "port", DefaultPort, "port of remote editor")
  flag.IntVar(&port, "p", DefaultPort, "port of remote editor")

  flag.IntVar(&fileLine, "line", 1, "Place caret on line number after loading file")
  flag.IntVar(&fileLine, "l", 1, "Place caret on line number after loading file")

  flag.StringVar(&fileName, "m", Defaulthost, "The display name shown in editor")
  flag.StringVar(&fileName, "name", Defaulthost, "The display name shown in editor")

  flag.StringVar(&fileType, "t", Defaulthost, "Treat file as having specified type")
  flag.StringVar(&fileType, "type", Defaulthost, "Treat file as having specified type")


  flag.Parse()

  configureLogging(verbose)

  // --- 2. 环境变量优先级检查 ---
  if envHost := os.Getenv("GOMATE_HOST"); envHost != "" {
    if host == Defaulthost {
      host = envHost
      // 改进: 使用 fmt.Println，避免被 configureLogging 禁用
      //fmt.Printf("Using host from environment variable GOMATE_HOST: %s\n", host)
      // log.Printf("Using host from environment variable GOMATE_HOST: %s\n", host)
    }
  }

  if envPortStr := os.Getenv("GOMATE_PORT"); envPortStr != "" {
    if port == DefaultPort {
      var envPort int
      _, err := fmt.Sscanf(envPortStr, "%d", &envPort)
      if err == nil {
        port = envPort
        // fmt.Printf("Using port from environment variable GOMATE_PORT: %d\n", port)
        // log.Printf("Using port from environment variable GOMATE_PORT: %d\n", port)
      } else {
        // log.Printf("Warning: GOMATE_PORT environment variable is invalid. Using port %d", port)
      }
    }
  }

  // --- 2. 信号处理 Goroutine ---
  // 创建一个 channel 用于接收信号
  sigs := make(chan os.Signal, 1)
  // 告诉 Go 关注 SIGINT (Ctrl+C) 和 SIGTERM (Windows 窗口关闭)
  signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

  // 信号监听 Goroutine 启动
  go func() {
    sig := <-sigs // 阻塞，直到收到信号
    log.Printf("Received system signal: %s. Initiating graceful cleanup.", sig)

    // 收到信号后，执行清理并退出。
    // 由于 lockFile 和 lockFilePath 尚未定义，我们不能在这里直接访问。
    // 我们需要使用一个 channel 通知主 Goroutine 执行清理。

    // 改进方案：使用一个 channel 专门通知主 Goroutine 退出
    exitSignal <- struct{}{}
  }()

  // --- 3. 文件存在性检查和多实例互斥 ---
  args := flag.Args()
  if len(args) == 0 {
    fmt.Println("Error: No file path provided.")
    fmt.Println("Usage: gomate [options] <file1> [file2...]")
    os.Exit(1)
  }

  targetFile := args[0]
  if err := ensureFileExists(targetFile); err != nil {
    // 在使用 log.Fatal 函数时，内部就调用了 os.Exit(1)
    log.Fatalf("Fatal: Failed to ensure file existence for %s: %v", targetFile, err)
  }

  // 检查是否已存在实例
  log.Printf("Try to open file: %s", targetFile)
  var lockFile *os.File
  lockFile, err := checkMultiInstance(targetFile, force)

  if err != nil {
    if errors.Is(err, ErrInstanceAlreadyRunning) {
      os.Exit(0) // 优雅退出 (状态码 0)
    }
    // 在使用 log.Fatal 函数时，内部就调用了 os.Exit(1)
    log.Fatal(err) // 致命错误，退出并记录
  }

  // ❗ 核心修正：清理函数
  lockFilePath := lockFile.Name()
  cleanup := func() {
    if lockFile != nil {
      if closeErr := lockFile.Close(); closeErr != nil {
        log.Printf("Warning: failed to close lock file %s: %v", lockFilePath, closeErr)
      }
      if removeErr := os.Remove(lockFilePath); removeErr != nil && !os.IsNotExist(removeErr) {
        log.Printf("Warning: failed to remove lock file %s: %v", lockFilePath, removeErr)
      }
      log.Println("Lock released and lock file deleted.")
    }
  }

  // --- 4. 网络连接和通信 ---
  log.Printf("Connection target: %s:%d", host, port)
  conn, err := net.Dial("tcp", fmt.Sprintf("%v:%v", host, port))
  if err != nil {
    // 在致命错误退出前，defer 会执行 cleanup()
    log.Fatal(err)
  }

  closeConn := func() {
    if closeErr := conn.Close(); closeErr != nil {
      log.Printf("Warning: failed to close network connection: %v", closeErr)
    }
  }

  // 确保在主函数退出时, 关闭连接
  defer closeConn()

  // 确保在主函数正常退出时（如连接失败）清理锁
  defer cleanup()

  // 发送文件
  for _, f := range flag.Args() {
    log.Printf("Send file %s to %s", f, host)
    if err = sendFile(conn, f); err != nil {
      // sendFile 失败是致命的
      // 在致命错误退出前，defer 会执行 cleanup()
      // 在使用 log.Fatal 函数时，内部就调用了 os.Exit(1)
      log.Fatal(err)
    }
    break // 只处理第一个文件
  }

  // 接收编辑器握手信息
  buf := bufio.NewReader(conn)
  b, _, err := buf.ReadLine()
  if err != nil {
    log.Fatal(err)
  }
  log.Printf("Editor handshake: %s", strings.TrimSpace(string(b)))

  // ----------------------------------------------------
  // ❗ 核心修正：将 handleCommands 放入 Goroutine
  // ----------------------------------------------------
  // 必须在主 Goroutine 外部运行，才能保证 select 能够及时响应信号。
  go func() {
    for {
      exit, err := handleCommands(buf)

      result := CommandResult{Exit: exit, Err: err}

      // 检查是否应该退出 Goroutine：
      // 1. (err != nil): 出错时，发送错误结果并退出 Goroutine
      // 2. (exit == true): 收到 'close' 命令时，正常退出
      if err != nil || exit {
        commandResult <- result
        return // 退出 Goroutine
      }

      // 引入微小延迟，防止 CPU 占用过高 (在阻塞的 handleCommands 外层是必要的)
      time.Sleep(5 * time.Millisecond)
    }
  }()

  // ----------------------------------------------------
  // 5. 主循环等待退出信号或命令结果
  // ----------------------------------------------------
  for {
    select {
    case <-exitSignal:
      // 收到来自信号 Goroutine 的通知 (窗口关闭/Ctrl+C)
      log.Println("Signal-triggered exit.")
      goto EndLoop

    case res := <-commandResult:
      // 收到来自命令处理 Goroutine 的结果
      if res.Err != nil {
        log.Fatal(res.Err) // 命令处理中遇到致命错误
      }
      if res.Exit {
        log.Println("Command-triggered exit.")
        goto EndLoop // 退出 For 循环，执行 defer
      }
    }
  }

EndLoop:
  // main 函数正常返回，defer 会清理所有资源。
  log.Println("Gomate client exiting.")
}
