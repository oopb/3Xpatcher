package mieru

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBinaryPath = "/usr/local/x-ui-mieru/bin/mita"
	DefaultConfigDir  = "/usr/local/x-ui-mieru/config"
	DefaultServiceFmt = "x-ui-mieru@%d.service"
)

type Runtime struct { BinaryPath, ConfigDir, ServiceFmt string }
func DefaultRuntime() Runtime { return Runtime{BinaryPath: DefaultBinaryPath, ConfigDir: DefaultConfigDir, ServiceFmt: DefaultServiceFmt} }
func (r Runtime) normalized() Runtime {
	if r.BinaryPath == "" { r.BinaryPath = DefaultBinaryPath }; if r.ConfigDir == "" { r.ConfigDir = DefaultConfigDir }; if r.ServiceFmt == "" { r.ServiceFmt = DefaultServiceFmt }; return r
}
func (r Runtime) configPath(id int) string { return filepath.Join(r.normalized().ConfigDir, strconv.Itoa(id)+".json") }
func (r Runtime) service(id int) string { return fmt.Sprintf(r.normalized().ServiceFmt, id) }
func (r Runtime) udsPath(id int) string { return filepath.Join("/run/x-ui-mieru", strconv.Itoa(id)+".sock") }

// CheckBytes performs the validations that are safe before swapping a live
// config. mita's `apply config` command is intentionally not used here: it is
// an online RPC operation and requires a running management UDS. Full parser,
// listener and traffic-pattern validation is verified after restart via mita's
// own RUNNING state, with automatic rollback on failure.
func (r Runtime) CheckBytes(config []byte) error {
	r = r.normalized()
	if !json.Valid(config) { return errors.New("Mieru renderer produced invalid JSON") }
	if _, err := os.Stat(r.BinaryPath); err != nil { return fmt.Errorf("mita binary: %w", err) }
	out, err := exec.Command(r.BinaryPath, "version").CombinedOutput()
	if err != nil { return fmt.Errorf("mita version check failed: %w: %s", err, strings.TrimSpace(string(out))) }
	return nil
}

func (r Runtime) Apply(id int, config []byte) error {
	if id <= 0 { return errors.New("invalid Mieru inbound id") }
	r = r.normalized(); if err := r.CheckBytes(config); err != nil { return err }; if err := os.MkdirAll(r.ConfigDir, 0o700); err != nil { return err }
	path := r.configPath(id); var old []byte; oldExists := false
	if b, err := os.ReadFile(path); err == nil { old, oldExists = b, true } else if !errors.Is(err, os.ErrNotExist) { return err }
	if oldExists && bytes.Equal(bytes.TrimSpace(old), bytes.TrimSpace(config)) {
		if err := r.Enable(id); err != nil { return err }
		if err := r.VerifyRunning(id, 2*time.Second); err == nil { return nil }
		return r.restartAndVerify(id)
	}
	tmp, err := os.CreateTemp(r.ConfigDir, fmt.Sprintf(".%d-*.json", id)); if err != nil { return err }; tmpName := tmp.Name(); defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil { tmp.Close(); return err }; if _, err := tmp.Write(config); err != nil { tmp.Close(); return err }; if err := tmp.Sync(); err != nil { tmp.Close(); return err }; if err := tmp.Close(); err != nil { return err }; if err := os.Rename(tmpName, path); err != nil { return err }
	if err := r.restartAndVerify(id); err != nil {
		if oldExists { _ = os.WriteFile(path, old, 0o600); _ = r.restartAndVerify(id) } else { _ = os.Remove(path); _ = r.Disable(id) }
		return fmt.Errorf("new Mieru config failed official mita startup validation; previous config restored: %w", err)
	}
	return nil
}

func (r Runtime) restartAndVerify(id int) error {
	if err := r.Restart(id); err != nil { return err }
	return r.VerifyRunning(id, 5*time.Second)
}
func (r Runtime) VerifyRunning(id int, timeout time.Duration) error {
	r = r.normalized(); deadline := time.Now().Add(timeout); var last string
	for {
		cmd := exec.Command(r.BinaryPath, "status")
		cmd.Env = append(os.Environ(), "MITA_CONFIG_JSON_FILE="+r.configPath(id), "MITA_UDS_PATH="+r.udsPath(id))
		out, err := cmd.CombinedOutput(); last = strings.TrimSpace(string(out))
		if err == nil && strings.Contains(last, `mita server status is "RUNNING"`) { return nil }
		if time.Now().After(deadline) { if last == "" { last = "no status output" }; return fmt.Errorf("mita instance %d did not reach RUNNING state: %s", id, last) }
		time.Sleep(100 * time.Millisecond)
	}
}
func (r Runtime) Enable(id int) error { out, err := exec.Command("systemctl", "enable", r.service(id)).CombinedOutput(); if err != nil { return fmt.Errorf("systemctl enable %s: %w: %s", r.service(id), err, strings.TrimSpace(string(out))) }; return nil }
func (r Runtime) Restart(id int) error { if err := r.Enable(id); err != nil { return err }; out, err := exec.Command("systemctl", "restart", r.service(id)).CombinedOutput(); if err != nil { return fmt.Errorf("systemctl restart %s: %w: %s", r.service(id), err, strings.TrimSpace(string(out))) }; return nil }
func (r Runtime) Stop(id int) error { out, err := exec.Command("systemctl", "stop", r.service(id)).CombinedOutput(); if err != nil { text := strings.TrimSpace(string(out)); if strings.Contains(text, "not loaded") || strings.Contains(text, "not found") { return nil }; return fmt.Errorf("systemctl stop %s: %w: %s", r.service(id), err, text) }; return nil }
func (r Runtime) Disable(id int) error { out, err := exec.Command("systemctl", "disable", "--now", r.service(id)).CombinedOutput(); if err != nil { text := strings.TrimSpace(string(out)); if strings.Contains(text, "not loaded") || strings.Contains(text, "not found") || strings.Contains(text, "does not exist") { return nil }; return fmt.Errorf("systemctl disable --now %s: %w: %s", r.service(id), err, text) }; return nil }
func (r Runtime) Remove(id int) error { if err := r.Disable(id); err != nil { return err }; if err := os.Remove(r.configPath(id)); err != nil && !errors.Is(err, os.ErrNotExist) { return err }; return nil }
func (r Runtime) Status(id int) (string, error) { out, err := exec.Command("systemctl", "is-active", r.service(id)).CombinedOutput(); status := strings.TrimSpace(string(out)); if status != "" { return status, nil }; return status, err }
func (r Runtime) ConfiguredIDs() ([]int, error) { r = r.normalized(); entries, err := os.ReadDir(r.ConfigDir); if errors.Is(err, os.ErrNotExist) { return nil, nil }; if err != nil { return nil, err }; ids := make([]int, 0); for _, entry := range entries { if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" { continue }; id, err := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".json")); if err == nil && id > 0 { ids = append(ids, id) } }; return ids, nil }
func (r Runtime) Version() (string, error) { r = r.normalized(); out, err := exec.Command(r.BinaryPath, "version").CombinedOutput(); if err != nil { return "", fmt.Errorf("mita version: %w: %s", err, strings.TrimSpace(string(out))) }; return strings.TrimSpace(string(out)), nil }
