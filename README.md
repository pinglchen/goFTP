# goFTP
a minimal ftp server as per section 5.1 of RFC 959.

由RFC 959的5.1节设计FTP服务器的最小实现

   TYPE - ASCII Non-print
   MODE - Stream
   STRUCTURE - File, Record
   COMMANDS - USER, QUIT, PORT,
              TYPE, MODE, STRU,
                for the default values
              RETR, STOR,
              NOOP.

        The default values for transfer parameters are:

           TYPE - ASCII Non-print
           MODE - Stream
           STRU - File
命令的相关描述：
LIST 如果指定了文件或目录，返回其信息；否则返回当前工作目录的信息
USER 认证用户名
QUIT 断开连接
PORT 指定服务器要连接的地址和端口
TYPE 设定传输模式（ASCII/二进制).
MODE 设定传输模式（流、块或压缩）
STRU 设定文件传输结构

RETR 传输文件副本
STOR 接收数据并且在服务器站点保存为文件
NOOP 无操作（哑包；通常用来保活）
SYST 返回系统类型
PASV 进入被动模式

实现中主要有一个结构体conn
```go
type conn struct {
	rw           net.Conn // "Protocol Interpreter" connection
	dataHostPort string
	prevCmd      string
	pasvListener net.Listener
	cmdErr       error // Saved command connection write error.
	binary       bool
}
```
[conn]()

并由此定义了一系列方法
日志
```go
func (c *conn) log(pairs logPairs)
```
数据连接
```go
func (c *conn) dataConn() (conn io.ReadWriteCloser, err error)
```
列出目录
```go
func (c *conn) list(args []string)
```
被动模式
```go
func (c *conn) pasv(args []string)
```
指定连接地址及端口
```go
func (c *conn) port(args []string)
```
设定传输模式（ASCII/二进制)
```go
func (c *conn) type_(args []string)
```
设定文件传输结构
```go
func (c *conn) stru(args []string)
```
传输文件副本
```go
func (c *conn) retr(args []string)
```
接收数据并且在服务器站点保存为文件
```go
func (c *conn) stor(args []string)
```


### `net` 包基础

`net` 包为网络 `I/O` 提供了一个便携式接口，包括 `TCP/IP`，`UDP`，域名解析和 `Unix` 域套接字。 

虽然该软件包提供对低级网络原语的访问，但大多数客户端只需要 `Dial`，`Listen` 和 `Accept` 函数以及相关的 `Conn` 和 `Listener` 接口提供的基本接口。

 拨号功能连接到服务器： 
```go
conn, err := net.Dial("tcp", "217.0.0.1:8080")
if err != nil {
	// 处理错误
}
fmt.Fprintf(conn, "GET / HTTP/1.0\r\n\r\n")
status, err := bufio.NewReader(conn).ReadString('\n')
// ...
```
 Listen函数创建服务器： 
 ```go
 ln, err := net.Listen("tcp", ":8080")
if err != nil {
	// 处理错误		
}
for {
	conn, err := ln.Accept()
	if err != nil {
		// 处理错误
	}
	go handleConnection(conn)
}
```


