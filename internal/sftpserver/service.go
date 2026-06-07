package sftpserver

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"

	"webssh/internal/sshclient"
)

type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	IsDir   bool      `json:"is_dir"`
	ModTime time.Time `json:"mod_time"`
}

type SFTPService struct{}

func NewSFTPService() *SFTPService {
	return &SFTPService{}
}

func (s *SFTPService) newClient(sshClient *sshclient.Client) (*sftp.Client, error) {
	if !sshClient.IsConnected() {
		return nil, fmt.Errorf("SSH 客户端未连接")
	}
	sess, err := sshClient.NewSession()
	if err != nil {
		return nil, err
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
	if err := sess.RequestSubsystem("sftp"); err != nil {
		sess.Close()
		return nil, fmt.Errorf("请求 SFTP 子系统失败: %w", err)
	}
	return sftp.NewClientPipe(stdout, stdin)
}

func (s *SFTPService) List(sshClient *sshclient.Client, path string) ([]FileInfo, error) {
	client, err := s.newClient(sshClient)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	files, err := client.ReadDir(path)
	if err != nil {
		return nil, err
	}

	result := make([]FileInfo, 0, len(files))
	for _, f := range files {
		fullPath := filepath.ToSlash(filepath.Join(path, f.Name()))
		mode := f.Mode()
		result = append(result, FileInfo{
			Name:    f.Name(),
			Path:    fullPath,
			Size:    f.Size(),
			Mode:    mode.String(),
			IsDir:   f.IsDir(),
			ModTime: f.ModTime(),
		})
	}
	return result, nil
}

func (s *SFTPService) Upload(sshClient *sshclient.Client, localPath, remotePath string, content io.Reader) error {
	client, err := s.newClient(sshClient)
	if err != nil {
		return err
	}
	defer client.Close()

	dst, err := client.Create(remotePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, content)
	return err
}

func (s *SFTPService) Download(sshClient *sshclient.Client, remotePath string) (io.ReadCloser, error) {
	client, err := s.newClient(sshClient)
	if err != nil {
		return nil, err
	}

	src, err := client.Open(remotePath)
	if err != nil {
		client.Close()
		return nil, err
	}
	return &sftpReadCloser{ReadCloser: src, client: client}, nil
}

type sftpReadCloser struct {
	io.ReadCloser
	client *sftp.Client
}

func (r *sftpReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.client.Close()
	return err
}

func (s *SFTPService) Delete(sshClient *sshclient.Client, path string, recursive bool) error {
	client, err := s.newClient(sshClient)
	if err != nil {
		return err
	}
	defer client.Close()

	if recursive {
		files, err := client.ReadDir(path)
		if err == nil {
			for _, f := range files {
				fullPath := filepath.ToSlash(filepath.Join(path, f.Name()))
				if f.IsDir() {
					if err := s.Delete(sshClient, fullPath, true); err != nil {
						return err
					}
				} else {
					if err := client.Remove(fullPath); err != nil {
						return err
					}
				}
			}
		}
	}
	return client.Remove(path)
}

func (s *SFTPService) Rename(sshClient *sshclient.Client, oldPath, newPath string) error {
	client, err := s.newClient(sshClient)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Rename(oldPath, newPath)
}

func (s *SFTPService) Mkdir(sshClient *sshclient.Client, path string) error {
	client, err := s.newClient(sshClient)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.MkdirAll(path)
}

func (s *SFTPService) Chmod(sshClient *sshclient.Client, path string, mode os.FileMode) error {
	client, err := s.newClient(sshClient)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Chmod(path, mode)
}

func (s *SFTPService) ReadFile(sshClient *sshclient.Client, path string) (string, error) {
	rc, err := s.Download(sshClient, path)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *SFTPService) WriteFile(sshClient *sshclient.Client, path, content string) error {
	client, err := s.newClient(sshClient)
	if err != nil {
		return err
	}
	defer client.Close()

	dst, err := client.Create(path)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = dst.Write([]byte(content))
	return err
}

func safeBaseName(p string) string {
	p = filepath.ToSlash(p)
	if idx := strings.LastIndex(p, "/"); idx >= 0 {
		return p[idx+1:]
	}
	return p
}
