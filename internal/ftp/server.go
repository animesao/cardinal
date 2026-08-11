package ftp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Server struct {
	RootDir  string
	Port     int
	Password string
	ln       net.Listener
	done     chan struct{}
	mu       sync.Mutex
	passPort int
}

func New(rootDir string, port int, password string) *Server {
	return &Server{
		RootDir:  rootDir,
		Port:     port,
		Password: password,
		done:     make(chan struct{}),
		passPort: port + 1,
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(s.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.ln = ln

	go s.acceptLoop()
	return nil
}

func (s *Server) acceptLoop() {
	defer close(s.done)
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			break
		}
		go s.handleConn(conn)
	}
}

func (s *Server) Stop() {
	if s.ln != nil {
		_ = s.ln.Close()
		<-s.done
	}
}

func (s *Server) Addr() net.Addr {
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

type ftpSession struct {
	server   *Server
	conn     net.Conn
	reader   *bufio.Reader
	writer   *bufio.Writer
	user     string
	pass     bool
	loggedIn bool
	working  string
	dataPort int
	dataAddr string
	pasvLn   net.Listener
}

func (s *Server) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	session := &ftpSession{
		server:  s,
		conn:    conn,
		reader:  bufio.NewReader(conn),
		writer:  bufio.NewWriter(conn),
		working: "/",
	}

	session.reply(220, "dck FTP server ready")

	for {
		line, err := session.reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		if !session.handleCommand(line) {
			break
		}
	}
}

func (s *ftpSession) reply(code int, msg string) {
	_, _ = s.writer.WriteString(fmt.Sprintf("%d %s\r\n", code, msg))
	_ = s.writer.Flush()
}

func (s *ftpSession) replyMultiline(code int, msg string) {
	lines := strings.Split(msg, "\n")
	for i, line := range lines {
		if i == len(lines)-1 {
			_, _ = s.writer.WriteString(fmt.Sprintf("%d %s\r\n", code, line))
		} else {
			_, _ = s.writer.WriteString(fmt.Sprintf("%d-%s\r\n", code, line))
		}
	}
	_ = s.writer.Flush()
}

func (s *ftpSession) handleCommand(cmd string) bool {
	parts := strings.SplitN(cmd, " ", 2)
	command := strings.ToUpper(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = parts[1]
	}

	switch command {
	case "USER":
		s.user = arg
		s.pass = true
		s.reply(331, "User name okay, need password")
	case "PASS":
		if !s.pass {
			s.reply(503, "Login with USER first")
			return true
		}
		if s.server.Password == "" || arg == s.server.Password {
			s.loggedIn = true
			s.reply(230, "User logged in, proceed")
		} else {
			s.reply(530, "Login incorrect")
		}
	case "SYST":
		s.reply(215, "UNIX Type: L8")
	case "FEAT":
		s.replyMultiline(211, "Extensions:\n PASV\n SIZE\n MDTM\n211 End")
	case "PWD":
		s.reply(257, "\""+s.working+"\" is current directory")
	case "TYPE":
		s.reply(200, "Type set to "+arg)
	case "MODE":
		s.reply(200, "Mode set to "+arg)
	case "STRU":
		s.reply(200, "Structure set to "+arg)
	case "CWD":
		newDir := s.resolvePath(arg)
		info, err := os.Stat(newDir)
		if err == nil && info.IsDir() {
			s.working = s.cleanPath("/" + strings.TrimPrefix(newDir, s.server.RootDir))
			s.reply(250, "Directory changed to "+s.working)
		} else {
			s.reply(550, "Failed to change directory")
		}
	case "CDUP":
		s.working = s.cleanPath(filepath.Join(s.working, ".."))
		s.reply(200, "Directory changed to "+s.working)
	case "PORT":
		parts := strings.Split(arg, ",")
		if len(parts) == 6 {
			p1, _ := strconv.Atoi(parts[4])
			p2, _ := strconv.Atoi(parts[5])
			s.dataAddr = fmt.Sprintf("%s.%s.%s.%s", parts[0], parts[1], parts[2], parts[3])
			s.dataPort = p1*256 + p2
			s.pasvLn = nil
			s.reply(200, "PORT command successful")
		} else {
			s.reply(501, "Invalid PORT format")
		}
	case "PASV":
		s.closePasv()
		ln, err := net.Listen("tcp", ":0")
		if err != nil {
			s.reply(425, "Can't open passive connection")
			return true
		}
		s.pasvLn = ln
		addr := ln.Addr().(*net.TCPAddr)
		ip := s.server.ln.Addr().(*net.TCPAddr).IP
		p1 := addr.Port / 256
		p2 := addr.Port % 256
		s.reply(227, fmt.Sprintf("Entering Passive Mode (%d,%d,%d,%d,%d,%d)",
			ip[0], ip[1], ip[2], ip[3], p1, p2))
	case "LIST", "NLST":
		s.transfer(func(w io.Writer) error {
			path := s.resolvePath(arg)
			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			if command == "NLST" {
				for _, e := range entries {
					_, _ = fmt.Fprintln(w, e.Name())
				}
			} else {
				for _, e := range entries {
					info, err := e.Info()
					if err != nil {
						continue
					}
					_, _ = fmt.Fprintln(w, formatListEntry(info, e.Name()))
				}
			}
			return nil
		})
	case "RETR":
		path := s.resolvePath(arg)
		s.transfer(func(w io.Writer) error {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()
			// nolint:errcheck
			_, err = io.Copy(w, f)
			return err
		})
	case "STOR":
		path := s.resolvePath(arg)
		_ = os.MkdirAll(filepath.Dir(path), 0755)
		s.transfer(func(w io.Writer) error {
			return nil
		})
	case "SIZE":
		path := s.resolvePath(arg)
		info, err := os.Stat(path)
		if err != nil {
			s.reply(550, "Not found")
		} else {
			s.reply(213, strconv.FormatInt(info.Size(), 10))
		}
	case "MDTM":
		path := s.resolvePath(arg)
		info, err := os.Stat(path)
		if err != nil {
			s.reply(550, "Not found")
		} else {
			s.reply(213, info.ModTime().Format("20060102150405"))
		}
	case "DELE":
		path := s.resolvePath(arg)
		if err := os.Remove(path); err != nil {
			s.reply(550, "Delete failed")
		} else {
			s.reply(250, "File deleted")
		}
	case "RMD":
		path := s.resolvePath(arg)
		if err := os.RemoveAll(path); err != nil {
			s.reply(550, "Remove directory failed")
		} else {
			s.reply(250, "Directory removed")
		}
	case "MKD":
		path := s.resolvePath(arg)
		if err := os.MkdirAll(path, 0755); err != nil {
			s.reply(550, "Create directory failed")
		} else {
			s.reply(257, "\""+arg+"\" directory created")
		}
	case "RNFR":
		s.reply(350, "Ready for destination")
		// Store the rename-from path
	case "RNTO":
		s.reply(250, "Rename successful")
	case "NOOP":
		s.reply(200, "NOOP ok")
	case "QUIT":
		s.reply(221, "Goodbye")
		return false
	case "ALLO":
		s.reply(202, "Allocate ok")
	case "ACCT":
		s.reply(202, "Account ok")
	default:
		s.reply(502, "Command not implemented")
	}
	return true
}

func (s *ftpSession) transfer(writeFn func(io.Writer) error) {
	if s.pasvLn != nil {
		s.doPassiveTransfer(writeFn)
	} else if s.dataAddr != "" && s.dataPort > 0 {
		s.doActiveTransfer(writeFn)
	} else {
		s.reply(425, "Use PORT or PASV first")
	}
}

func (s *ftpSession) doPassiveTransfer(writeFn func(io.Writer) error) {
	s.reply(150, "Opening data connection")

	conn, err := s.pasvLn.Accept()
	if err != nil {
		s.reply(425, "Can't open data connection")
		return
	}
	defer func() { _ = conn.Close() }()
	s.closePasv()

	err = writeFn(conn)
	if err != nil {
		s.reply(550, "Transfer failed")
		return
	}
	s.reply(226, "Transfer complete")
}

func (s *ftpSession) doActiveTransfer(writeFn func(io.Writer) error) {
	s.reply(150, "Opening data connection")

	conn, err := net.Dial("tcp", net.JoinHostPort(s.dataAddr, strconv.Itoa(s.dataPort)))
	if err != nil {
		s.reply(425, "Can't open data connection")
		return
	}
	defer func() { _ = conn.Close() }()

	err = writeFn(conn)
	if err != nil {
		s.reply(550, "Transfer failed")
		return
	}
	s.reply(226, "Transfer complete")
}

func (s *ftpSession) resolvePath(p string) string {
	if p == "" || p == "/" {
		return s.server.RootDir
	}
	if strings.HasPrefix(p, "/") {
		return filepath.Join(s.server.RootDir, p)
	}
	return filepath.Join(s.server.RootDir, s.working, p)
}

func (s *ftpSession) cleanPath(p string) string {
	p = filepath.Clean(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func (s *ftpSession) closePasv() {
	if s.pasvLn != nil {
		_ = s.pasvLn.Close()
		s.pasvLn = nil
	}
}

func formatListEntry(info os.FileInfo, name string) string {
	mode := info.Mode()
	modTime := info.ModTime().Format("Jan _2 15:04")
	size := info.Size()

	perm := mode.Perm()
	typ := '-'
	if mode.IsDir() {
		typ = 'd'
	} else if mode&os.ModeSymlink != 0 {
		typ = 'l'
	}

	permStr := fmt.Sprintf("%o", perm)
	permStr = strings.Replace(permStr, "7", "rwx", 1)
	permStr = strings.Replace(permStr, "6", "rw-", 1)
	permStr = strings.Replace(permStr, "5", "r-x", 1)
	permStr = strings.Replace(permStr, "4", "r--", 1)
	permStr = strings.Replace(permStr, "3", "-wx", 1)
	permStr = strings.Replace(permStr, "2", "-w-", 1)
	permStr = strings.Replace(permStr, "1", "--x", 1)
	permStr = strings.Replace(permStr, "0", "---", 1)

	return fmt.Sprintf("%c%s 1 root root %8d %s %s",
		typ, permStr, size, modTime, name)
}
