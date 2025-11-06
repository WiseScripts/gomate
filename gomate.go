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
	"path/filepath"
	"strconv"
	"strings"
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

	// 改进: defer 语句中应记录 f.Close() 的错误
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

	// 遵循 remote_subl 协议写入头部信息
	// 改进: 使用 log.Printf 记录发送信息，但仅在 verbose 模式下可见
	log.Printf("Sending file header for: %s (Token: %s, Size: %d)", filename, hash, st.Size())

	fmt.Fprintf(conn, "open\n")
	fmt.Fprintf(conn, "display-name: %v\n", filepath.Base(filename))
	fmt.Fprintf(conn, "real-path: %v\n", filename)
	fmt.Fprintf(conn, "data-on-save: yes\n")
	fmt.Fprintf(conn, "re-activate: yes\n")
	fmt.Fprintf(conn, "token: %v\n", hash)
	fmt.Fprintf(conn, "data: %v\n", st.Size())

	// 改进: io.Copy 失败时不应使用 log.Fatal，应返回错误
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
		log.Println("Received 'close' command. Exiting gracefully.")
		return true, nil

	case "save":
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

// checkMultiInstance 尝试在全局目录下创建独占锁文件。
func checkMultiInstance(filePath string) (*os.File, error) {
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path: %w", err)
	}
	log.Printf("Absolute path: %s", absFilePath)

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

	// 3. 构造最终的锁文件路径
	absFilePathHash := fmt.Sprintf("%x", md5.Sum([]byte(lowCaseAbsFilePath)))
	lockFilePath := filepath.Join(globalLockDir, absFilePathHash)
	log.Printf("Lock file path: %s", lockFilePath)

	// 4. 尝试独占创建锁文件
	// O_CREATE | O_EXCL: 确保文件独占创建，如果已存在则返回 os.ErrExist
	lockFile, err := os.OpenFile(lockFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		if os.IsExist(err) {
			log.Printf("File already exists: %s. An instance is already running.", lockFilePath)
			// ❗ 改进: 返回自定义错误，让 main 优雅退出 (状态码 0)
			return nil, ErrInstanceAlreadyRunning
		} else {
			// 其它创建文件时的致命错误
			return nil, fmt.Errorf("error creating lock file: %w", err)
		}
	} else {
		log.Printf("Created lock file successfully: %s", lockFilePath)
		// 锁定成功
		return lockFile, nil
	}
}

func main() {
	// --- 1. 参数定义和解析 ---
	const DefaultHostname = "localhost"
	const DefaultPort = 52698

	var hostname string
	var port int
	var verbose bool

	flag.BoolVar(&verbose, "v", false, "Enable verbose logging output")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging output")
	flag.StringVar(&hostname, "hostname", DefaultHostname, "hostname of remote editor")
	flag.IntVar(&port, "port", DefaultPort, "port of remote editor")
	flag.Parse()

	configureLogging(verbose)

	// --- 2. 环境变量优先级检查 ---
	if envHost := os.Getenv("GOMATE_HOST"); envHost != "" {
		if hostname == DefaultHostname {
			hostname = envHost
			// 改进: 使用 fmt.Println，避免被 configureLogging 禁用
			//fmt.Printf("Using hostname from environment variable GOMATE_HOST: %s\n", hostname)
			// log.Printf("Using hostname from environment variable GOMATE_HOST: %s\n", hostname)
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

	// --- 3. 文件存在性检查和多实例互斥 ---
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Error: No file path provided.")
		fmt.Println("Usage: gomate [options] <file1> [file2...]")
		os.Exit(1)
	}

	targetFile := args[0]
	if err := ensureFileExists(targetFile); err != nil {
		log.Fatalf("Fatal: Failed to ensure file existence for %s: %v", targetFile, err)
	}

	// 检查是否已存在实例
	log.Printf("Try to open file %s", targetFile)
	var lockFile *os.File
	lockFile, err := checkMultiInstance(targetFile)

	// ❗ 改进: 优雅退出和致命错误分离
	if err != nil {
		if errors.Is(err, ErrInstanceAlreadyRunning) {
			os.Exit(0) // 优雅退出 (状态码 0)
		}
		log.Fatal(err) // 致命错误，退出并记录
	}

	// ❗ 改进: 确保 defer 逻辑正确关闭和删除锁文件
	lockFilePath := lockFile.Name()
	defer func() {
		if lockFile != nil {
			if closeErr := lockFile.Close(); closeErr != nil {
				log.Printf("Warning: failed to close lock file %s: %v", lockFilePath, closeErr)
			}
			if removeErr := os.Remove(lockFilePath); removeErr != nil && !os.IsNotExist(removeErr) {
				log.Printf("Warning: failed to remove lock file %s: %v", lockFilePath, removeErr)
			}
			log.Println("Lock released and lock file deleted.")
		}
	}()

	// --- 4. 网络连接和通信 ---
	log.Printf("Connection target: %s:%d", hostname, port)
	conn, err := net.Dial("tcp", fmt.Sprintf("%v:%v", hostname, port))
	if err != nil {
		log.Fatal(err)
	}
	// 改进: defer 中应记录错误
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("Warning: failed to close network connection: %v", closeErr)
		}
	}()

	// 发送文件
	for _, f := range flag.Args() {
		// log.Printf("Send file %s to :%s", f, hostname)
		// fmt.Printf("Send file %s to :%s", f, hostname)
		if err = sendFile(conn, f); err != nil {
			log.Fatal(err) // sendFile 失败是致命的
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

	// 循环处理命令
	var exit bool
	for {
		exit, err = handleCommands(buf)
		if err != nil {
			log.Fatal(err) // 命令处理中遇到致命错误
		}
		if exit {
			break
		}
        // ----------------------------------------------------
        // 引入微小延迟，防止 CPU 占用过高
        // ----------------------------------------------------
        // 让出 CPU，给其他 goroutine 或系统进程执行的机会
        time.Sleep(5 * time.Millisecond)		
	}

	// main 函数正常返回，defer 会清理所有资源。
}
