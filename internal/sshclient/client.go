package sshclient

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	ErrNotConnected = errors.New("SSH 客户端未连接")
)

type Config struct {
	Host       string
	Port       int
	Username   string
	Password   string
	PrivateKey string
	Passphrase string
	AuthType   string
	Timeout    int
	KeepAlive  int
}

type Client struct {
	config    *Config
	sshClient *ssh.Client
	mu        sync.RWMutex
	closed    bool
}

func NewClient(cfg *Config) *Client {
	return &Client{config: cfg}
}

func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	timeout := time.Duration(c.config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	authMethods, err := c.buildAuthMethods()
	if err != nil {
		return err
	}

	sshConfig := &ssh.ClientConfig{
		User:            c.config.Username,
		Auth:            authMethods,
		Timeout:         timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SSH 握手失败: %w", err)
	}

	c.sshClient = ssh.NewClient(sshConn, chans, reqs)
	c.closed = false

	if c.config.KeepAlive > 0 {
		go c.keepAlive()
	}

	return nil
}

func (c *Client) buildAuthMethods() ([]ssh.AuthMethod, error) {
	methods := []ssh.AuthMethod{}
	if c.config.Password != "" {
		methods = append(methods, ssh.Password(c.config.Password))
	}
	if c.config.PrivateKey != "" {
		signer, err := c.parsePrivateKey()
		if err != nil {
			return nil, fmt.Errorf("私钥解析失败: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if len(methods) == 0 {
		return nil, errors.New("至少需要一种认证方式")
	}
	return methods, nil
}

func (c *Client) parsePrivateKey() (ssh.Signer, error) {
	if c.config.Passphrase != "" {
		return ssh.ParsePrivateKeyWithPassphrase([]byte(c.config.PrivateKey), []byte(c.config.Passphrase))
	}
	return ssh.ParsePrivateKey([]byte(c.config.PrivateKey))
}

func (c *Client) keepAlive() {
	ticker := time.NewTicker(time.Duration(c.config.KeepAlive) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.RLock()
		if c.closed || c.sshClient == nil {
			c.mu.RUnlock()
			return
		}
		client := c.sshClient
		c.mu.RUnlock()

		if _, _, err := client.SendRequest("keepalive@webssh", true, nil); err != nil {
			c.Close()
			return
		}
	}
}

func (c *Client) NewSession() (*ssh.Session, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed || c.sshClient == nil {
		return nil, ErrNotConnected
	}
	return c.sshClient.NewSession()
}

func (c *Client) SSHClient() *ssh.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sshClient
}

func (c *Client) NewShellSession(cols, rows int, term string) (*Session, error) {
	sess, err := c.NewSession()
	if err != nil {
		return nil, err
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if term == "" {
		term = "xterm-256color"
	}

	if err := sess.RequestPty(term, rows, cols, modes); err != nil {
		sess.Close()
		return nil, fmt.Errorf("请求 PTY 失败: %w", err)
	}

	stdin, err := sess.StdinPipe()
	if err != nil {
		sess.Close()
		return nil, err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		sess.Close()
		return nil, err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		sess.Close()
		return nil, err
	}

	if err := sess.Shell(); err != nil {
		sess.Close()
		return nil, fmt.Errorf("启动 shell 失败: %w", err)
	}

	return &Session{
		Session: sess,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
		cols:    cols,
		rows:    rows,
		term:    term,
	}, nil
}

func (c *Client) ListenTCP(host string, port int) (net.Listener, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed || c.sshClient == nil {
		return nil, ErrNotConnected
	}
	return c.sshClient.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
}

func (c *Client) DialTCP(host string, port int) (net.Conn, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed || c.sshClient == nil {
		return nil, ErrNotConnected
	}
	return c.sshClient.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.sshClient != nil {
		return c.sshClient.Close()
	}
	return nil
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.closed && c.sshClient != nil
}

type Session struct {
	*ssh.Session
	Stdin  io.WriteCloser
	Stdout io.Reader
	Stderr io.Reader
	cols   int
	rows   int
	term   string
	mu     sync.Mutex
}

func (s *Session) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Stdin.Write(p)
}

func (s *Session) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cols <= 0 || rows <= 0 {
		return nil
	}
	s.cols = cols
	s.rows = rows
	return s.Session.WindowChange(rows, cols)
}
