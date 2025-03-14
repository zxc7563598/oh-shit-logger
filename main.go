package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const dataDir = "data" // 数据存储目录
const port = 9999      // 服务端口
const days = 7         // 数据保留天数

var mu sync.Mutex

// 定义日志数据结构
type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Context struct {
		IP      string `json:"ip"`
		Method  string `json:"method"`
		FullURL string `json:"full_url"`
		Trace   struct {
			Message string `json:"message"`
			File    string `json:"file"`
			Line    int    `json:"line"`
			Trace   []struct {
				File string `json:"file"`
				Line int    `json:"line"`
			} `json:"trace"`
		} `json:"trace"`
	} `json:"context"`
}

// 获取当前日期的文件名
func getFileName() string {
	return filepath.Join(dataDir, "data_"+time.Now().Format("2006-01-02")+".txt")
}

// 写入数据到文件
func writeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只允许POST方法", http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "读取请求正文失败", http.StatusInternalServerError)
		return
	}

	// 直接将 JSON 数据作为一行追加到文件中
	mu.Lock()
	defer mu.Unlock()

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		http.Error(w, "无法创建数据目录", http.StatusInternalServerError)
		return
	}

	file, err := os.OpenFile(getFileName(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "无法打开文件", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if _, err := file.Write(body); err != nil {
		http.Error(w, "写入文件失败", http.StatusInternalServerError)
		return
	}
	if _, err := file.WriteString("\n"); err != nil {
		http.Error(w, "写入文件失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "数据保存成功"})
}

// 读取数据并渲染 HTML 页面
func readHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "只允许使用GET方法", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	date := query.Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02") // 默认显示当天的数据
	}

	mu.Lock()
	defer mu.Unlock()

	// 读取指定日期的文件
	filePath := filepath.Join(dataDir, "data_"+date+".txt")
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte{} // 文件不存在时返回空数据
		} else {
			http.Error(w, "读取文件失败", http.StatusInternalServerError)
			return
		}
	}

	// 解析每行 JSON 数据
	var logEntries []LogEntry
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			fmt.Printf("无法解析 JSON 数据: %s, 错误: %v\n", line, err)
			continue
		}
		logEntries = append(logEntries, entry)
	}

	// 加载 HTML 模板
	tmpl, err := template.ParseFiles("templates/read.html")
	if err != nil {
		http.Error(w, "无法加载模板", http.StatusInternalServerError)
		return
	}

	// 渲染模板并注入数据
	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, map[string]interface{}{
		"LogEntries": logEntries,
		"Date":       date,
	})
}

// 删除指定行的数据
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "只允许使用DELETE方法", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	lineStr := query.Get("line")
	if lineStr == "" {
		http.Error(w, "行号是必需的", http.StatusBadRequest)
		return
	}

	lineNum, err := strconv.Atoi(lineStr)
	if err != nil || lineNum < 1 {
		http.Error(w, "行号无效", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// 读取指定日期的文件
	date := time.Now().Format("2006-01-02")
	filePath := filepath.Join(dataDir, "data_"+date+".txt")
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		http.Error(w, "读取文件失败", http.StatusInternalServerError)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if lineNum > len(lines) {
		http.Error(w, "行号超出范围", http.StatusBadRequest)
		return
	}

	// 删除指定行
	newLines := append(lines[:lineNum-1], lines[lineNum:]...)
	if err := ioutil.WriteFile(filePath, []byte(strings.Join(newLines, "\n")+"\n"), 0644); err != nil {
		http.Error(w, "写入文件失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Line deleted successfully"})
}

// 自动清理过期数据
func cleanupOldData(days int) {
	files, err := ioutil.ReadDir(dataDir)
	if err != nil {
		fmt.Println("无法读取数据目录:", err)
		return
	}

	now := time.Now()
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// 解析文件名中的日期
		fileName := file.Name()
		if !strings.HasPrefix(fileName, "data_") || !strings.HasSuffix(fileName, ".txt") {
			continue
		}
		dateStr := strings.TrimPrefix(strings.TrimSuffix(fileName, ".txt"), "data_")
		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			fmt.Printf("无法解析文件名中的日期: %s\n", fileName)
			continue
		}

		// 删除超过指定天数的文件
		if now.Sub(fileDate).Hours() > float64(days*24) {
			filePath := filepath.Join(dataDir, fileName)
			if err := os.Remove(filePath); err != nil {
				fmt.Printf("无法删除文件: %s, 错误: %v\n", filePath, err)
			} else {
				fmt.Printf("已删除过期文件: %s\n", filePath)
			}
		}
	}
}

func main() {
	// 启动定时任务，每天清理一次过期数据
	go func() {
		for {
			cleanupOldData(days)
			time.Sleep(24 * time.Hour)
		}
	}()
	http.HandleFunc("/write", writeHandler)
	http.HandleFunc("/read", readHandler)
	http.HandleFunc("/delete", deleteHandler)
	fmt.Printf("Server started at :%d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
	}
}
