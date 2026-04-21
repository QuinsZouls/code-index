package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

type DaemonInfo struct {
	PID         int       `json:"pid"`
	ProjectRoot string    `json:"project_root"`
	StartedAt   time.Time `json:"started_at"`
	Interval    string    `json:"interval"`
	Debounce    string    `json:"debounce"`
}

func registryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".codeindex")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemons.json"), nil
}

func lockFilePath(projectRoot string) string {
	hash := fileHash([]byte(projectRoot))
	return filepath.Join(os.TempDir(), "codeindex-"+hash[:16]+".lock")
}

func loadRegistry() ([]DaemonInfo, error) {
	path, err := registryPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []DaemonInfo{}, nil
		}
		return nil, err
	}
	var daemons []DaemonInfo
	if err := json.Unmarshal(data, &daemons); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	return daemons, nil
}

func saveRegistry(daemons []DaemonInfo) error {
	path, err := registryPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(daemons, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func addDaemon(info DaemonInfo) error {
	daemons, err := loadRegistry()
	if err != nil {
		return err
	}
	for i, d := range daemons {
		if d.ProjectRoot == info.ProjectRoot {
			if isProcessAlive(d.PID) {
				return fmt.Errorf("daemon already running for this project (PID %d)", d.PID)
			}
			daemons = append(daemons[:i], daemons[i+1:]...)
			break
		}
	}
	daemons = append(daemons, info)
	return saveRegistry(daemons)
}

func removeDaemon(pid int) error {
	daemons, err := loadRegistry()
	if err != nil {
		return err
	}
	for i, d := range daemons {
		if d.PID == pid {
			daemons = append(daemons[:i], daemons[i+1:]...)
			return saveRegistry(daemons)
		}
	}
	return nil
}

func findDaemonByProject(root string) (*DaemonInfo, error) {
	daemons, err := loadRegistry()
	if err != nil {
		return nil, err
	}
	for i := range daemons {
		if daemons[i].ProjectRoot == root {
			return &daemons[i], nil
		}
	}
	return nil, nil
}

func isProcessAlive(pid int) bool {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid))
		output, err := cmd.CombinedOutput()
		if err != nil {
			return false
		}
		return len(output) > 0
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func stopProcess(pid int) error {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F")
		return cmd.Run()
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}

func acquireLock(projectRoot string) (func(), error) {
	lockPath := lockFilePath(projectRoot)
	data, err := os.ReadFile(lockPath)
	if err == nil {
		var existingPID int
		if _, err := fmt.Sscanf(string(data), "%d", &existingPID); err == nil {
			if isProcessAlive(existingPID) {
				return nil, fmt.Errorf("another daemon is already running (PID %d)", existingPID)
			}
		}
	}
	lockContent := fmt.Sprintf("%d\n", os.Getpid())
	if err := os.WriteFile(lockPath, []byte(lockContent), 0o644); err != nil {
		return nil, err
	}
	return func() {
		os.Remove(lockPath)
	}, nil
}

func cleanDeadDaemons() error {
	daemons, err := loadRegistry()
	if err != nil {
		return err
	}
	var alive []DaemonInfo
	for _, d := range daemons {
		if isProcessAlive(d.PID) {
			alive = append(alive, d)
		} else {
			lockPath := lockFilePath(d.ProjectRoot)
			os.Remove(lockPath)
		}
	}
	if len(alive) == len(daemons) {
		return nil
	}
	return saveRegistry(alive)
}
