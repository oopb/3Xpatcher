package singbox

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	DefaultBinaryPath = "/usr/local/x-ui-singbox/bin/sing-box"
	DefaultConfigPath = "/usr/local/x-ui-singbox/config/config.json"
	DefaultService    = "x-ui-singbox.service"
)

type Runtime struct {
	BinaryPath string
	ConfigPath string
	Service    string
}

func DefaultRuntime() Runtime {
	return Runtime{BinaryPath: DefaultBinaryPath, ConfigPath: DefaultConfigPath, Service: DefaultService}
}

func (r Runtime) normalized() Runtime {
	if r.BinaryPath == "" {
		r.BinaryPath = DefaultBinaryPath
	}
	if r.ConfigPath == "" {
		r.ConfigPath = DefaultConfigPath
	}
	if r.Service == "" {
		r.Service = DefaultService
	}
	return r
}

func (r Runtime) CheckBytes(config []byte) error {
	r = r.normalized()
	if _, err := os.Stat(r.BinaryPath); err != nil {
		return fmt.Errorf("sing-box binary: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(r.ConfigPath), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(r.ConfigPath), ".check-*.json")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(config); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return r.checkFile(name)
}

func (r Runtime) checkFile(path string) error {
	cmd := exec.Command(r.BinaryPath, "check", "-c", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box check failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Apply validates first, atomically swaps the config, restarts only sing-box,
// and restores the previous config if the restart fails. Xray is never touched.
func (r Runtime) Apply(config []byte) error {
	r = r.normalized()
	if err := r.CheckBytes(config); err != nil {
		return err
	}
	dir := filepath.Dir(r.ConfigPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	var old []byte
	oldExists := false
	if b, err := os.ReadFile(r.ConfigPath); err == nil {
		old, oldExists = b, true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(config); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, r.ConfigPath); err != nil {
		return err
	}
	cleanup = false

	if err := r.Restart(); err != nil {
		if oldExists {
			_ = os.WriteFile(r.ConfigPath, old, 0o600)
			_ = r.Restart()
		} else {
			_ = os.Remove(r.ConfigPath)
			_ = r.Stop()
		}
		return fmt.Errorf("new sing-box config passed check but restart failed; previous config restored: %w", err)
	}
	return nil
}

func (r Runtime) Restart() error {
	r = r.normalized()
	out, err := exec.Command("systemctl", "restart", r.Service).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart %s: %w: %s", r.Service, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (r Runtime) Stop() error {
	r = r.normalized()
	out, err := exec.Command("systemctl", "stop", r.Service).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl stop %s: %w: %s", r.Service, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (r Runtime) Status() (string, error) {
	r = r.normalized()
	out, err := exec.Command("systemctl", "is-active", r.Service).CombinedOutput()
	status := strings.TrimSpace(string(out))
	// systemctl returns a non-zero exit status for a valid inactive/failed state.
	// Surface that state to the UI instead of turning the status endpoint itself
	// into an error. An empty result still means systemctl could not answer.
	if status != "" {
		return status, nil
	}
	return status, err
}

func (r Runtime) Version() (string, error) {
	r = r.normalized()
	out, err := exec.Command(r.BinaryPath, "version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sing-box version: %w: %s", err, strings.TrimSpace(string(out)))
	}
	first, _, _ := bytes.Cut(out, []byte("\n"))
	return strings.TrimSpace(string(first)), nil
}
