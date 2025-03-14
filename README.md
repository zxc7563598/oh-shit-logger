# 完蛋日志💥

### **为什么有这个破玩意？**

因为 bug 太多，日志太乱，服务器太远，人在加班，心态崩盘。  
所以我写了这么个**倒霉玩意**，用来收集远程日志，免得我老是去翻服务器。

### **它能干嘛？**

**别指望它很强，它就干三件事**：

1. **收垃圾** 🚮 —— 你发日志过来，它记一笔，存文件里，爱咋咋地。
2. **翻垃圾** 🗑 —— 想看日志了，访问 `/read`​，所有倒霉事儿一览无遗。
3. **删垃圾** ❌ —— 看到烦人的错误信息？`DELETE /delete?line=N`​ 直接删掉，眼不见心不烦。

### **怎么用？**

1. 启动它（就这破玩意，能跑就行）：

    ```bash
    go run main.go
    ```
2. 让你的项目 POST 日志过来：

    ```bash
    curl -X POST -d "error=服务器又双叒叕炸了" http://localhost:8080/write
    ```
3. 访问 `/read`​ 查看日志：

    ```bash
    curl http://localhost:8080/read
    ```
4. 删除某一行日志：

    ```bash
    curl -X DELETE "http://localhost:8080/delete?line=2"
    ```

### **结语**

反正就是个临时抱佛脚的东西，我没心情做得多优雅，能用就行。  
如果你也在被 bug 折磨，不妨用它来当垃圾桶，至少你可以把屎山日志收集到一个地方，而不是每次 SSH 上去翻半天。

 **—— 写这破玩意的时候，我想休假 🫠**
