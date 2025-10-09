# 🧾 FinBoard Backend

A lightweight error log collection and analysis web application written in Go, designed to centrally manage error logs from multiple PHP projects ~~(other projects are fine too, but most of mine are PHP, so that's why I say it this way)~~ and leverage **DeepSeek** to help you quickly pinpoint issues.

> Detect and resolve problems **before your clients even notice**.

**This project has been parsed by Zread. If you need a quick overview of the project, you can click here to view it：[Understand this project](https://zread.ai/zxc7563598/oh-shit-logger)**

---

## ✨ Motivation

As someone maintaining multiple PHP projects, you may have experienced these frustrations:

- Issues only become apparent after client feedback, making you reactive rather than proactive;
- When everything seems fine, it’s hard to proactively check through all logs.

This is why I created **oh-shit-logger**.  
It aggregates all your project’s critical errors on a single page, organizes them by date, supports pagination, and can automatically analyze error information using **DeepSeek** to provide possible solutions.

By deploying it on your own server (or a company machine), all PHP project error logs can be reported in real time.  
This way, even before the client speaks up, you already know: "Oh, something went wrong."

| List                                                                                          | Details                                                                                     |
| --------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| ​<img src="https://raw.githubusercontent.com/zxc7563598/oh-shit-logger/main/demo/0001.png"> ​ | ​<img src="https://raw.githubusercontent.com/zxc7563598/oh-shit-logger/main/demo/0002.png"> |

---

## 🚀 Deployment

### 📦 Option 1: Use the Precompiled Binary

Download the latest release from **[Releases](https://github.com/zxc7563598/oh-shit-logger/releases)**, extract it, and run:

```bash
chmod +x ./app
./app -port=9999 -retain=7 -user=admin -pass=123123
```

**Available parameters:**

| Parameter  | Description                      | Default  |
| ---------- | -------------------------------- | -------- |
| ​`port`​   | Port number to run the server    | ​`9999`​ |
| ​`retain`​ | Number of days to retain logs    | ​`7`​    |
| ​`user`​   | BasicAuth username (recommended) | -        |
| ​`pass`​   | BasicAuth password (recommended) | -        |

> If both `user` and `pass` are not provided, BasicAuth will be disabled. It’s **not recommended** to disable authentication to prevent unauthorized access to error logs.

After starting, access:  
👉 `http://YOUR_SERVER_IP:PORT/read`  
to view error logs.

---

### 🧰 Option 2: Build from Source

Clone the project to your local machine or server:

```bash
git clone https://github.com/zxc7563598/oh-shit-logger.git ./oh-shit-logger
```

Build the project:

```bash
cd oh-shit-logger
go build -o ./app main.go
```

Run the project (same as above):

```bash
./app -port=9999
```

> Access `http://YOUR_SERVER_IP:PORT/read` to see the error list.

---

## 🐘 How to Use in PHP

In your project’s exception handling logic, format errors into a unified structure and report them:

> Place this in your central exception handler, regardless of framework.

```php
/**
 * Format a Throwable as a standard JSON string
 * POST the data to your server’s /write endpoint, e.g.: http://YOUR_SERVER_IP:PORT/write
 *
 * @param Throwable $e Error object
 * @param array $context Optional contextual information to help locate issues
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
        'project'   => 'project', // Your project name
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

// Report the error to oh-shit-logger
$ch = curl_init();
curl_setopt($ch, CURLOPT_URL, 'http://YOUR_SERVER_IP:PORT/write');
curl_setopt($ch, CURLOPT_POST, true);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
curl_setopt($ch, CURLOPT_POSTFIELDS, formatThrowable($exception, [
    'module' => 'user-center',
    'action' => 'login',
]));
curl_exec($ch);
curl_close($ch);
```

> It is recommended to report **only fatal or unrecoverable exceptions**, or focus on critical errors to avoid log overload.

---

## 🔍 Viewing & Analysis

Visit `http://YOUR_SERVER_IP:PORT/read`  
to view all reported error logs.

- Automatically categorized by date
- Supports pagination
- Built-in DeepSeek analysis for one-click issue insights

---

## 🤝 Contributing

Feel free to submit issues or feature requests via **[Issues](https://github.com/zxc7563598/oh-shit-logger/issues)**.  
If this project helps you, **please don’t forget to ⭐️ Star to show your support!**
