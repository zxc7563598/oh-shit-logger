package main

import (
	"encoding/json"
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
	dataDir     = "data"        // 数据存储目录
	port        = 9999          // 服务端口
	retainDays  = 7             // 数据保留天数
	timeFormat  = "2006-01-02"  // 日期格式
	fileNameFmt = "data_%s.txt" // 文件名格式
	authUser    = "admin"       // 账号，记得改
	authPass    = "123123"      // 密码，记得改
)

var rwMu sync.RWMutex // 读写锁优化并发性能

type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Context struct {
		Project string `json:"project"`
		IP      string `json:"ip"`
		Method  string `json:"method"`
		FullURL string `json:"full_url"`
		Trace   struct {
			Class   string       `json:"class"`
			Message string       `json:"message"`
			Code    int          `json:"code"`
			File    string       `json:"file"`
			Line    int          `json:"line"`
			Trace   []StackFrame `json:"trace"`
		} `json:"trace"`
	} `json:"context"`
}

type StackFrame struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function,omitempty"`
	Class    string `json:"class,omitempty"`
	Type     string `json:"type,omitempty"`
}

func getFileName(date time.Time) string {
	return filepath.Join(dataDir, fmt.Sprintf(fileNameFmt, date.Format(timeFormat)))
}

func writeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		logError(r, "读取请求正文失败", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 验证JSON格式
	var tmp LogEntry
	if err := json.Unmarshal(body, &tmp); err != nil {
		logError(r, "无效的JSON格式", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	rwMu.Lock()
	defer rwMu.Unlock()

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logError(r, "创建目录失败", err)
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	filename := getFileName(time.Now().UTC())
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logError(r, "打开文件失败", err)
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if _, err := file.Write(append(body, '\n')); err != nil {
		logError(r, "写入文件失败", err)
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]string{"status": "success"})
}

func readHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().UTC().Format(timeFormat)
	}

	date, err := time.Parse(timeFormat, dateStr)
	if err != nil {
		http.Error(w, "无效的日期格式", http.StatusBadRequest)
		return
	}

	rwMu.RLock()
	defer rwMu.RUnlock()

	filePath := getFileName(date)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte{}
		} else {
			logError(r, "读取文件失败", err)
			http.Error(w, "Internal Error", http.StatusInternalServerError)
			return
		}
	}

	var entries []LogEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			logError(r, "JSON解析失败", err)
			continue
		}
		entries = append(entries, entry)
	}

	tmpl, err := template.ParseFiles("templates/read.html")
	if err != nil {
		logError(r, "模板加载失败", err)
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, map[string]interface{}{
		"LogEntries": entries,
		"Date":       dateStr,
	}); err != nil {
		logError(r, "模板渲染失败", err)
	}
}

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

		if now.Sub(fileDate).Hours() > float64(retainDays*24) {
			filePath := filepath.Join(dataDir, name)
			if err := os.Remove(filePath); err != nil {
				logError(nil, "清理任务-删除文件失败", err)
			} else {
				fmt.Printf("已清理过期文件: %s\n", filePath)
			}
		}
	}
}

func logError(r *http.Request, msg string, err error) {
	logStr := fmt.Sprintf("[ERROR] %s - %v", msg, err)
	if r != nil {
		logStr = fmt.Sprintf("%s [%s %s]", logStr, r.Method, r.URL.Path)
	}
	fmt.Println(logStr)
}

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logError(nil, "JSON响应失败", err)
	}
}

func basicAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != authUser || pass != authPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

func main() {
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

	// 启动服务器
	serverAddr := fmt.Sprintf(":%d", port)
	fmt.Printf("服务器启动于 %s\n", serverAddr)
	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		panic(fmt.Sprintf("服务器启动失败: %v", err))
	}
}
