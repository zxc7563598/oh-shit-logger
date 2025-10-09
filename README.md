# 完蛋日志 💥

一个用 Go 编写的轻量级错误日志收集与分析网站，用来集中管理多个 PHP 项目~~(其他项目也可以，都可以，只不过我的大部分项目都是 PHP 所以我这么说)~~的错误日志，并借助 **DeepSeek** 帮助你更快定位问题。

> 在客户找上门之前，先一步发现并解决问题。

---

## ✨ 初衷

作为一个需要维护多个 PHP 项目的人，你可能也经历过这样的困境：

- 问题出现了，客户反馈后才发现，显得非常被动；
- 没问题的时候又很难主动去翻各个日志；

于是我写了这个项目 —— **oh-shit-logger**。  
它可以把你所有项目的致命错误都汇总在一个页面中，按日期分类，支持分页浏览，还能通过 **DeepSeek** 自动分析错误信息，为你提供解决思路。

只要部署在自己的服务器（或公司的一台机器上），所有 PHP 项目的错误日志都能实时上报。  
这样，当客户还没开口，你就已经知道“哦，出事了”。

| 列表                                                                                          | 详情                                                                                        |
| --------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| ​<img src="https://raw.githubusercontent.com/zxc7563598/oh-shit-logger/main/demo/0001.png"> ​ | ​<img src="https://raw.githubusercontent.com/zxc7563598/oh-shit-logger/main/demo/0002.png"> |

---

## 🚀 部署说明

### 📦 方式一：直接使用编译版本

在 **[Releases](https://github.com/zxc7563598/oh-shit-logger/releases)** 中下载最新版本到服务器，解压后进入目录执行：

```bash
chmod +x ./app
./app -port=9999 -retain=7 -user=admin -pass=123123
```

**可用参数说明：**

| 参数       | 说明                         | 默认值   |
| ---------- | ---------------------------- | -------- |
| ​`port`​   | 运行端口号                   | ​`9999`​ |
| ​`retain`​ | 日志保留天数                 | ​`7`​    |
| ​`user`​   | BasicAuth 用户名（建议设置） | -        |
| ​`pass`​   | BasicAuth 密码（建议设置）   | -        |

> 当 `user` 与 `pass` 均不传递时，将关闭 BasicAuth 认证，不建议关闭认证，避免错误信息被不相关的人看到

启动后访问：  
👉 `http://您的服务器IP:端口号/read`  
即可查看错误信息。

---

### 🧰 方式二：自行构建

同步项目到本地或服务器：

```bash
git clone https://github.com/zxc7563598/oh-shit-logger.git ./oh-shit-logger
```

构建项目：

```bash
cd oh-shit-logger
go build -o ./app main.go
```

运行项目（与上方一致）：

```bash
./app -port=9999
```

> 启动后访问 `http://您的服务器IP:端口号/read` 查看错误列表。

---

## 🐘 如何在 PHP 中使用

在各项目的异常处理逻辑中，将错误信息格式化为统一结构并上报：

> 添加在异常处理的位置，不管什么框架总该要有一个统一的异常处理类

```php
/**
 * 格式化 Throwable 为标准 JSON 字符串
 * 将返回的数据 POST 到您的服务器 /write 接口，例如：http://您的服务器IP:端口号/write
 *
 * @param Throwable $e 错误对象
 * @param array $context 可选的上下文信息，用于帮助定位问题
 */
function formatThrowable(Throwable $e, array $context = []): string
{
    $trace = array_map(static function ($t) {
        return [
            'file'     => $t['file'] ?? null,
            'line'     => $t['line'] ?? null,
            'function' => $t['function'] ?? null,
            'class'    => $t['class'] ?? null,
        ];
    }, $e->getTrace() ?? []);

    $data = [
        'uuid'      => bin2hex(random_bytes(8)),
        'project'   => 'bilibili-danmu', // 你的项目名
        'level'     => 'error',
        'timestamp' => date('c'),
        'message'   => $e->getMessage(),
        'code'      => $e->getCode(),
        'file'      => $e->getFile(),
        'line'      => $e->getLine(),
        'trace'     => $trace,
        'context'   => (object)$context,
        'server'    => [
            'hostname'    => gethostname() ?: 'unknown',
            'ip'          => $_SERVER['SERVER_ADDR'] ?? '127.0.0.1',
            'php_version' => PHP_VERSION,
        ],
    ];
    return json_encode($data, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES);
}

// 将错误上报到 oh-shit-logger
$ch = curl_init();
curl_setopt($ch, CURLOPT_URL, 'http://您的服务器IP:端口号/write');
curl_setopt($ch, CURLOPT_POST, true);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
curl_setopt($ch, CURLOPT_POSTFIELDS, formatThrowable($exception, [
    'module' => 'user-center',
    'action' => 'login',
]));
curl_exec($ch);
curl_close($ch);
```

> 建议仅上报 **致命错误或不可恢复异常**，或者重点关注的？以避免日志过量

---

## 🔍 查看与分析

访问 `http://您的服务器IP:端口号/read`  
即可查看所有已上报的错误日志。

- 按日期自动分类存储
- 支持分页加载
- 内置 DeepSeek 分析，一键生成问题分析思路

---

## 🤝 参与贡献

欢迎通过 **[Issues](https://github.com/zxc7563598/oh-shit-logger/issues)** 反馈问题或提出新功能建议。  
如果这个项目帮到了你，**请别忘了点个 ⭐️ Star 支持一下！**
