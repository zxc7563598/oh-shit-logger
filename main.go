package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	dataDir     = "data"
	timeFormat  = "2006-01-02"
	fileNameFmt = "data_%s.txt"
	configFile  = "config.json"
)

var (
	Port        int
	RetainDays  int
	AuthUser    string
	AuthPass    string
	rwMu        sync.RWMutex
	logTemplate = template.Must(
		template.New("read.html").
			Funcs(template.FuncMap{
				"add": func(a, b int) int { return a + b },
				"sub": func(a, b int) int { return a - b },
				"toJson": func(v any) template.JS {
					b, _ := json.Marshal(v)
					return template.JS(b)
				},
			}).
			ParseFiles("templates/read.html"),
	)
)

type LogEntry struct {
	UUID      string                 `json:"uuid"`
	Project   string                 `json:"project"`
	Level     string                 `json:"level"`
	Timestamp string                 `json:"timestamp"`
	Message   string                 `json:"message"`
	Code      int                    `json:"code"`
	File      string                 `json:"file"`
	Line      int                    `json:"line"`
	Trace     []TraceFrame           `json:"trace"`
	Context   map[string]interface{} `json:"context"`
	Server    ServerInfo             `json:"server"`
}

type TraceFrame struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
	Class    string `json:"class"`
}

type ServerInfo struct {
	Hostname   string `json:"hostname"`
	IP         string `json:"ip"`
	PHPVersion string `json:"php_version"`
}

func getFileName(date time.Time) string {
	return filepath.Join(dataDir, fmt.Sprintf(fileNameFmt, date.Format(timeFormat)))
}

// POST /write - 写入日志
func writeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	// 解析 JSON
	var entry LogEntry
	if err := json.Unmarshal(body, &entry); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	// 转成格式化 JSON 字符串（可读性好，也可以改为压缩）
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// 确保 logs 目录存在
	if err := os.MkdirAll("logs", 0755); err != nil {
		http.Error(w, "Failed to create logs dir: "+err.Error(), http.StatusInternalServerError)
		return
	}
	filename := getFileName(time.Now().UTC())
	// 加锁写入文件（防止多个请求同时写）
	rwMu.Lock()
	defer rwMu.Unlock()
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "Failed to open file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	// 每条日志写入一行，方便后续处理（JSON Lines 格式）
	if _, err := f.WriteString(string(jsonBytes) + "\n"); err != nil {
		http.Error(w, "Failed to write file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "success"})
}

// GET /read - 读取日志
func readHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	// 参数处理
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().UTC().Format(timeFormat)
	}
	page := parseInt(r.URL.Query().Get("page"), 1)
	pageSize := parseInt(r.URL.Query().Get("page_size"), 100)
	if pageSize <= 0 {
		pageSize = 100
	}
	date, err := time.Parse(timeFormat, dateStr)
	if err != nil {
		http.Error(w, "无效的日期格式，应为 YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	// 读锁保护文件
	rwMu.RLock()
	defer rwMu.RUnlock()
	filePath := getFileName(date)
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			renderTemplate(w, dateStr, nil, page, 0, pageSize)
			return
		}
		logError(r, "打开文件失败", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var entries []LogEntry
	var (
		lineCount = 0
		start     = (page - 1) * pageSize
		end       = page * pageSize
	)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if lineCount >= start && lineCount < end {
			var entry LogEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				logError(r, "JSON解析失败", err)
			} else {
				entries = append(entries, entry)
			}
		}
		lineCount++
		if lineCount >= end {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		logError(r, "扫描文件失败", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	renderTemplate(w, dateStr, entries, page, lineCount, pageSize)
}

// 辅助方法 - 渲染 HTML 模板
func renderTemplate(w http.ResponseWriter, dateStr string, entries []LogEntry, page, total, pageSize int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := logTemplate.Execute(w, map[string]any{
		"LogEntries": entries,
		"Date":       dateStr,
		"Page":       page,
		"Total":      total,
		"PageSize":   pageSize,
		"HasNext":    len(entries) == pageSize, // 是否还有下一页
	})
	if err != nil {
		logError(nil, "模板渲染失败", err)
		http.Error(w, "Template Render Error", http.StatusInternalServerError)
	}
}

// 辅助方法 - 字符串转int
func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// DELETE /delete - 删除指定日志行
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query()
	dateStr := query.Get("date")
	if dateStr == "" {
		dateStr = time.Now().UTC().Format(timeFormat)
	}
	date, err := time.Parse(timeFormat, dateStr)
	if err != nil {
		http.Error(w, "无效的日期格式", http.StatusBadRequest)
		return
	}
	lineNum, err := strconv.Atoi(query.Get("line"))
	if err != nil || lineNum < 1 {
		http.Error(w, "无效的行号", http.StatusBadRequest)
		return
	}
	rwMu.Lock()
	defer rwMu.Unlock()
	filePath := getFileName(date)
	data, err := os.ReadFile(filePath)
	if err != nil {
		logError(r, "读取文件失败", err)
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if lineNum > len(lines) {
		http.Error(w, "行号超出范围", http.StatusBadRequest)
		return
	}
	newLines := append(lines[:lineNum-1], lines[lineNum:]...)
	tmpPath := filePath + ".tmp"
	content := strings.Join(newLines, "\n") + "\n"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		logError(r, "写入临时文件失败", err)
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		logError(r, "文件重命名失败", err)
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "success"})
}

// 定时任务 - 定期清理过期日志文件
func cleanupOldData() {
	rwMu.Lock()
	defer rwMu.Unlock()
	files, err := os.ReadDir(dataDir)
	if err != nil {
		logError(nil, "清理任务-读取目录失败", err)
		return
	}
	now := time.Now().UTC()
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		if !strings.HasPrefix(name, "data_") || !strings.HasSuffix(name, ".txt") {
			continue
		}
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, "data_"), ".txt")
		fileDate, err := time.Parse(timeFormat, dateStr)
		if err != nil {
			logError(nil, "清理任务-解析日期失败", err)
			continue
		}
		if now.Sub(fileDate).Hours() > float64(RetainDays*24) {
			filePath := filepath.Join(dataDir, name)
			if err := os.Remove(filePath); err != nil {
				logError(nil, "清理任务-删除文件失败", err)
			} else {
				fmt.Printf("已清理过期文件: %s\n", filePath)
			}
		}
	}
}

// 辅助方法 - 统一错误日志输出
func logError(r *http.Request, msg string, err error) {
	logStr := fmt.Sprintf("[ERROR] %s - %v", msg, err)
	if r != nil {
		logStr = fmt.Sprintf("%s [%s %s]", logStr, r.Method, r.URL.Path)
	}
	fmt.Println(logStr)
}

// 辅助方法 - JSON 响应
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logError(nil, "JSON响应失败", err)
	}
}

// 基本认证中间件
func basicAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if AuthUser == "" && AuthPass == "" {
			h(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != AuthUser || pass != AuthPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

type Config struct {
	APIKey string `json:"api_key"`
}

// GET /apikey - 获取当前 API Key
func getAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	var cfg Config
	data, err := os.ReadFile(configFile)
	if err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	resp := map[string]string{"api_key": cfg.APIKey}
	json.NewEncoder(w).Encode(resp)
}

// POST /apikey - 保存新的 API Key
func saveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var req map[string]string
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "读取请求体失败", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "JSON 解析失败", http.StatusBadRequest)
		return
	}
	key, ok := req["api_key"]
	if !ok {
		http.Error(w, "缺少 api_key 字段", http.StatusBadRequest)
		return
	}
	cfg := Config{APIKey: key}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(configFile, data, 0600); err != nil {
		http.Error(w, "保存失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	flag.IntVar(&Port, "port", 9999, "服务端口")
	flag.IntVar(&RetainDays, "retain", 7, "日志保留天数")
	flag.StringVar(&AuthUser, "user", "", "登录账号")
	flag.StringVar(&AuthPass, "pass", "", "登录密码")
	flag.Parse()
	// 初始化清理任务
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			cleanupOldData()
			<-ticker.C
		}
	}()
	// 创建必要目录
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		panic(fmt.Sprintf("无法创建数据目录: %v", err))
	}
	// 配置路由
	http.HandleFunc("/write", writeHandler)
	http.HandleFunc("/read", basicAuth(readHandler))
	http.HandleFunc("/delete", deleteHandler)
	http.HandleFunc("/apikey", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getAPIKey(w, r)
		case http.MethodPost:
			saveConfig(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})
	// 启动服务器
	serverAddr := fmt.Sprintf(":%d", Port)
	fmt.Printf("服务器启动于 %s\n", serverAddr)
	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		panic(fmt.Sprintf("服务器启动失败: %v", err))
	}
}
