# 完蛋日志 💥

### **为什么有这个破玩意？**

因为 bug 太多，日志太乱，服务器太远，人在加班，心态崩盘。  
所以我写了这么个**倒霉玩意**，用来收集远程日志，免得我老是去翻服务器。

这个破脚本是基于我自己的 [php-tools](https://github.com/zxc7563598/php-tools) 实现的日志记录系统。

我把它扔到服务器上，让项目里那些要命的异常能通过网络偷偷通知我，争取在老板冲出来骂我之前，先把锅给悄悄补上。

也许未来我会给它加上邮件通知功能……也许吧。但说实话，我有点担心我的邮箱会被这些破 bug 直接轰炸成垃圾场

### **它能干嘛？**

**别指望它很强，它就干三件事**：

1. **收垃圾** 🚮 —— 你发日志过来，它记一笔，存文件里，爱咋咋地。
2. **翻垃圾** 🗑 —— 想看日志了，访问 `/read`​，所有倒霉事儿一览无遗。
3. **删垃圾** ❌ —— 看到烦人的错误信息？`DELETE /delete?line=N`​ 直接删掉，眼不见心不烦。

### **怎么用？**

1. 把代码拉到你自己的服务器上

   ```bash
    cd /usr/local && git clone https://github.com/zxc7563598/oh-shit-logger.git
   ```

   > 也许你会考虑更换一下端口，或者日志保留天数，在 main.go 中

2. 编译你的 Go 脚本：

   ```bash
   go build -o oh-shit-logger main.go
   ```

3. 创建一个 `systemd` 服务文件：

   ```bash
   sudo nano /etc/systemd/system/oh-shit-logger.service
   ```

4. 在文件中添加以下内容：

   ```ini
   [Unit]
   Description=oh shit logger
   After=network.target

   [Service]
   ExecStart=/usr/local/oh-shit-logger/oh-shit-logger
   Restart=always
   User=root
   WorkingDirectory=/usr/local/oh-shit-logger

   [Install]
   WantedBy=multi-user.target
   ```

   - `ExecStart`：指定 Go 程序的路径。
   - `Restart=always`：如果程序崩溃，自动重启。
   - `User`：运行程序的用户（例如 `ubuntu`）。
   - `WorkingDirectory`：程序的工作目录。

5. 保存并退出编辑器，然后重新加载 `systemd` 配置：

   ```bash
   sudo systemctl daemon-reload
   ```

6. 启动服务：

   ```bash
   sudo systemctl start oh-shit-logger
   ```

7. 设置开机自启动：

   ```bash
   sudo systemctl enable oh-shit-logger
   ```

8. 检查服务状态：

   ```bash
   sudo systemctl status oh-shit-logger
   ```

9. 停止服务：

   ```bash
   sudo systemctl stop oh-shit-logger
   ```

10. 访问 `/read`​ 查看日志：

    ```bash
    curl http://服务器ip:端口号/read
    ```

11. 删除某一行日志：

    ```bash
    curl -X DELETE "http://服务器ip:端口号/delete?line=2"
    ```

- 其他的项目也可以直接 POST 日志过来，大概格式如下：

  ```bash
  curl -X POST http://localhost:8080/write -H "Content-Type: application/json" -d '{
      "time": "2025-05-20 09:58:31",
      "level": "ERROR",
      "message": "Division by zero",
      "context": {
          "project": "ToolsApi",
          "ip": "127.0.0.1",
          "method": "GET",
          "full_url": "//127.0.0.1:8787/test",
          "trace": {
              "class": "DivisionByZeroError",
              "message": "Division by zero",
              "code": 0,
              "file": "/Users/lisiqi/Documents/ToolsApi/config/route.php",
              "line": 25,
              "trace": [
                  {
                      "file": "/Users/lisiqi/Documents/ToolsApi/vendor/workerman/webman-framework/src/App.php",
                      "line": 150,
                      "function": "{closure}",
                      "class": "Webman\\Route",
                      "type": "::"
                  },
                  {
                      "file": "/Users/lisiqi/Documents/ToolsApi/vendor/workerman/workerman/src/Connection/TcpConnection.php",
                      "line": 741,
                      "function": "onMessage",
                      "class": "Webman\\App",
                      "type": "->"
                  }
              ]
          }
      }
  }'
  ```

### **结语**

反正就是个临时抱佛脚的东西，我没心情做得多优雅，能用就行。  
如果你也在被 bug 折磨，不妨用它来当垃圾桶，至少你可以把屎山日志收集到一个地方，而不是每次 SSH 上去翻半天。

**—— 写这破玩意的时候，我想休假 🫠**
