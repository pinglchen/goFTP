// a minimal ftp server as per section 5.1 of RFC 959.

//下面是FTP服务器的最小实现：
//
//类型 - ASCII Non-print
//
//模式 - Stream
//
//结构 - File, Record
//
//命令 - USER, QUIT, PORT,TYPE, MODE, STRU,RETR, STOR,NOOP.
//
//传输的默认参数为：
//
//类型 - ASCII Non-print
//
//模式 - Stream
//
//结构 - File
//
//所有主机都将上面的值作为默认值。
//
//   TYPE - ASCII Non-print
//   MODE - Stream
//   STRUCTURE - File, Record
//   COMMANDS - USER, QUIT, PORT,
//              TYPE, MODE, STRU,
//                for the default values
//              RETR, STOR,
//              NOOP.
//
//        The default values for transfer parameters are:
//
//           TYPE - ASCII Non-print
//           MODE - Stream
//           STRU - File
//

//LIST 如果指定了文件或目录，返回其信息；否则返回当前工作目录的信息
//USER 认证用户名
//QUIT 断开连接
//PORT 指定服务器要连接的地址和端口
//TYPE 设定传输模式（ASCII/二进制).
//MODE 设定传输模式（流、块或压缩）
//STRU 设定文件传输结构
//
//RETR 传输文件副本
//STOR 接收数据并且在服务器站点保存为文件
//NOOP 无操作（哑包；通常用来保活）
//SYST 返回系统类型
//PASV 进入被动模式


package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

type conn struct {
	rw           net.Conn // "Protocol Interpreter" connection
	dataHostPort string
	prevCmd      string
	pasvListener net.Listener
	cmdErr       error // Saved command connection write error.
	binary       bool
}

func NewConn(cmdConn net.Conn) *conn {
	return &conn{rw: cmdConn}
}

// hostPortToFTP returns a comma-separated, FTP-style address suitable for
// replying to the PASV command.
func hostPortToFTP(hostport string) (addr string, err error) {
	host, portStr, err := net.SplitHostPort(hostport)
	if err != nil {
		return "", err
	}
	//IPAddr 结构体的主要作用是用于域名解析服务 (DNS)，例如，函数 ResolveIPAddr() 可以通过主机名解析主机网络地址。
	ipAddr, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		return "", err
	}
	port, err := strconv.ParseInt(portStr, 10, 64)
	if err != nil {
		return "", err
	}
	ip := ipAddr.IP.To4()
	s := fmt.Sprintf("%d,%d,%d,%d,%d,%d", ip[0], ip[1], ip[2], ip[3], port/256, port%256)
	return s, nil
}

func hostPortFromFTP(address string) (string, error) {
	var a, b, c, d byte
	var p1, p2 int
	_, err := fmt.Sscanf(address, "%d,%d,%d,%d,%d,%d", &a, &b, &c, &d, &p1, &p2)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%d.%d.%d:%d", a, b, c, d, 256*p1+p2), nil
}

type logMap map[string]interface{}

func (c *conn) log(mps logMap) {
	b := &bytes.Buffer{}
	fmt.Fprintf(b, "addr=%s", c.rw.RemoteAddr().String())
	for k, v := range mps {
		fmt.Fprintf(b, " %s=%s", k, v)
	}
	log.Print(b.String())
}

func (c *conn) dataConn() (conn io.ReadWriteCloser, err error) { //数据连接 data connection
	switch c.prevCmd {
	case "PORT":
		conn, err = net.Dial("tcp", c.dataHostPort)
		if err != nil {
			return nil, err
		}
	case "PASV":
		conn, err = c.pasvListener.Accept()
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("previous command not PASV or PORT")
	}
	return conn, nil
}

// list prints file information to a data connection specified by the
// immediately preceding PASV or PORT command.
func (c *conn) list(args []string) {
	var filename string
	switch len(args) {
	case 0:
		filename = "."
	case 1:
		filename = args[0]
	default:
		c.writeln("501 Too many arguments.")
		return
	}
	file, err := os.Open(filename)
	if err != nil {
		c.writeln("550 File not found.")
		return
	}
	c.writeln("150 Here comes the directory listing.")
	w, err := c.dataConn()
	if err != nil {
		c.writeln("425 Can't open data connection.")
		return
	}
	defer w.Close()
	stat, err := file.Stat()
	if err != nil {
		c.log(logMap{"cmd": "LIST", "err": err})
		c.writeln("450 Requested file action not taken. File unavailable.")
	}
	// TODO: Print more than just the filenames.
	if stat.IsDir() {
		filenames, err := file.Readdirnames(0)
		if err != nil {
			c.writeln("550 Can't read directory.")
			return
		}
		for _, f := range filenames {
			_, err = fmt.Fprint(w, f, c.lineEnding())
			if err != nil {
				c.log(logMap{"cmd": "LIST", "err": err})
				c.writeln("426 Connection closed: transfer aborted.")
				return
			}
		}
	} else {
		_, err = fmt.Fprint(w, filename, c.lineEnding())
		if err != nil {
			c.log(logMap{"cmd": "LIST", "err": err})
			c.writeln("426 Connection closed: transfer aborted.")
			return
		}
	}
	c.writeln("226 Closing data connection. List successful.")
}

func (c *conn) writeln(s ...interface{}) {
	if c.cmdErr != nil {
		return
	}
	s = append(s, "\r\n")
	_, c.cmdErr = fmt.Fprint(c.rw, s...)
}

func (c *conn) lineEnding() string {
	if c.binary {
		return "\n"
	} else {
		return "\r\n"
	} //CR CRLF  Carriage-Return Line-Feed
}

func (c *conn) CmdErr() error {
	return c.cmdErr
}

func (c *conn) Close() error {
	err := c.rw.Close()
	if err != nil {
		c.log(logMap{"err": fmt.Errorf("closing command connection: %s", err)})
	}
	return err
}

func (c *conn) pasv(args []string) {
	if len(args) > 0 {
		c.writeln("501 Too many arguments.")
		return
	}
	var firstError error
	storeFirstError := func(err error) {
		if firstError == nil {
			firstError = err
		}
	}
	var err error
	c.pasvListener, err = net.Listen("tcp4", "") // ""
	storeFirstError(err)
	_, port, err := net.SplitHostPort(c.pasvListener.Addr().String())
	storeFirstError(err)
	ip, _, err := net.SplitHostPort(c.rw.LocalAddr().String())
	storeFirstError(err)
	addr, err := hostPortToFTP(fmt.Sprintf("%s:%s", ip, port))
	storeFirstError(err)
	if firstError != nil {
		c.pasvListener.Close()
		c.pasvListener = nil
		c.log(logMap{"cmd": "PASV", "err": err})
		c.writeln("451 Requested action aborted. Local error in processing.")
		return
	}
	// DJB recommends putting an extra character before the address.
	c.writeln(fmt.Sprintf("227 =%s", addr))
}

func (c *conn) port(args []string) {
	if len(args) != 1 {
		c.writeln("501 Usage: PORT a,b,c,d,p1,p2")
		return
	}
	var err error
	c.dataHostPort, err = hostPortFromFTP(args[0])  //dataHostPort
	if err != nil {
		c.log(logMap{"cmd": "PORT", "err": err})
		c.writeln("501 Can't parse address.")
		return
	}
	c.writeln("200 PORT command successful.")
}

func (c *conn) type_(args []string) {
	if len(args) < 1 || len(args) > 2 {
		c.writeln("501 Usage: TYPE takes 1 or 2 arguments.")
		return
	}
	switch strings.ToUpper(strings.Join(args, " ")) {
	case "A", "A N":
		c.binary = false
	case "I", "L 8":
		c.binary = true
	default:
		c.writeln("504 Unsupported type. Supported types: A, A N, I, L 8.")
		return
	}
	c.writeln("200 TYPE set")
}

func (c *conn) stru(args []string) {
	if len(args) != 1 {
		c.writeln("501 Usage: STRU F")
		return
	}
	if args[0] != "F" {
		c.writeln("504 Only file structure is supported")
		return
	}
	c.writeln("200 STRU set")
}

func (c *conn) retr(args []string) {
	if len(args) != 1 {
		c.writeln("501 Usage: RETR filename")
		return
	}
	filename := args[0]
	file, err := os.Open(filename)
	if err != nil {
		c.log(logMap{"cmd": "RETR", "err": err})
		c.writeln("550 File not found.")
		return
	}
	c.writeln("150 File ok. Sending.")
	conn, err := c.dataConn()
	if err != nil {
		c.writeln("425 Can't open data connection")
		return
	}
	defer conn.Close()
	if c.binary {
		_, err := io.Copy(conn, file)
		if err != nil {
			c.log(logMap{"cmd": "RETR", "err": err})
			c.writeln("450 File unavailable.")
			return
		}
	} else {
		// Convert line endings LF -> CRLF.
		r := bufio.NewReader(file)
		w := bufio.NewWriter(conn)
		for {
			line, isPrefix, err := r.ReadLine()
			if err != nil {
				if err == io.EOF {
					break
				}
				c.log(logMap{"cmd": "RETR", "err": err})
				c.writeln("450 File unavailable.")
				return
			}
			w.Write(line)
			if !isPrefix {
				w.Write([]byte("\r\n"))
			}
		}
		w.Flush() //Flush() 方法的功能是把缓冲区中的数据写入底层的 io.Writer，并返回错误信息。
	}
	c.writeln("226 Transfer complete.")
}

func (c *conn) stor(args []string) {
	if len(args) != 1 {
		c.writeln("501 Usage: STOR filename")
		return
	}
	filename := args[0]
	file, err := os.Create(filename)
	if err != nil {
		c.log(logMap{"cmd": "STOR", "err": err})
		c.writeln("550 File can't be created.")
		return
	}
	c.writeln("150 Ok to send data.")
	conn, err := c.dataConn()
	if err != nil {
		c.writeln("425 Can't open data connection")
		return
	}
	defer conn.Close()
	_, err = io.Copy(file, conn)
	if err != nil {
		c.log(logMap{"cmd": "RETR", "err": err})
		c.writeln("450 File unavailable.")
		return
	}
	c.writeln("226 Transfer complete.")
}

func (c *conn) run() {
	c.writeln("220 Ready.")
	input := bufio.NewScanner(c.rw)
	var cmd string
	var args []string
	for input.Scan() {
		if c.CmdErr() != nil {
			c.log(logMap{"err": fmt.Errorf("command connection: %s", c.CmdErr())})
			return
		}
		fields := strings.Fields(input.Text())
		if len(fields) == 0 {
			continue
		}
		cmd = strings.ToUpper(fields[0])
		args = nil
		if len(fields) > 1 {
			args = fields[1:]
		}
		switch cmd {
		case "LIST":
			c.list(args)
		case "NOOP":
			c.writeln("200 Ready.")
		case "PASV":
			c.pasv(args)
		case "PORT":
			c.port(args)
		case "QUIT":
			c.writeln("221 Goodbye.")
			return
		case "RETR":
			c.retr(args)
		case "STOR":
			c.stor(args)
		case "STRU":
			c.stru(args)
		case "SYST":
			// DJB recommends always replying with this string, to be
			// consistent with other servers and avoid weird fallback modes in
			// some clients.
			c.writeln("215 UNIX Type: L8")
		case "TYPE":
			c.type_(args)
		case "USER":
			c.writeln("230 Login successful.")
		default:
			c.writeln(fmt.Sprintf("502 Command %q not implemented.", cmd))
		}
		// Cleanup PASV listeners if they go unused.
		if cmd != "PASV" && c.pasvListener != nil {
			c.pasvListener.Close()
			c.pasvListener = nil
		}
		c.prevCmd = cmd
	}
	if input.Err() != nil {
		c.log(logMap{"err": fmt.Errorf("scanning commands: %s", input.Err())})
	}
}

func main() {
	var port int
	flag.IntVar(&port, "port", 8000, "listen port")  //-port=8000 flag.Parse()进行解析

	listen, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal("Opening main listener:", err)
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Print("Accepting new connection:", err)
		}
		go NewConn(conn).run()
	}
}
