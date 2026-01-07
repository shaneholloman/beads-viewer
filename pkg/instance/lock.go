// Package instance provides multi-instance coordination for bv.
// It detects when multiple bv instances are running on the same repository
// and provides mechanisms for coordination and user feedback.
package instance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// LockInfo contains metadata about the process holding the lock.
type LockInfo struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Hostname  string    `json:"hostname,omitempty"`
}

// Lock represents an instance lock for a beads directory.
// It uses a lock file to detect multiple concurrent instances.
type Lock struct {
	path     string
	lockFile *os.File
	pid      int
	isFirst  bool
}

// LockFileName is the name of the lock file created in .beads directory.
const LockFileName = ".bv.lock"

// NewLock creates a new instance lock for the given beads directory.
// If this is the first instance, it creates and holds the lock.
// If another instance already holds the lock, it returns a Lock with isFirst=false.
func NewLock(beadsDir string) (*Lock, error) {
	lockPath := filepath.Join(beadsDir, LockFileName)

	// Try to create lock file with exclusive access
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			// Lock file exists - read it to see who owns it
			existing, readErr := readLockFile(lockPath)
			lock := &Lock{
				path:    lockPath,
				isFirst: false,
			}
			if readErr == nil {
				lock.pid = existing.PID
			}

			// Check if the existing lock is stale
			lock.checkStale()

			return lock, nil
		}
		return nil, fmt.Errorf("creating lock file: %w", err)
	}

	// We got the lock - write our info
	lock := &Lock{
		path:     lockPath,
		lockFile: file,
		isFirst:  true,
		pid:      os.Getpid(),
	}
	if err := lock.writeLockInfo(); err != nil {
		file.Close()
		os.Remove(lockPath)
		return nil, fmt.Errorf("writing lock info: %w", err)
	}

	return lock, nil
}

// writeLockInfo writes the current process info to the lock file.
func (l *Lock) writeLockInfo() error {
	if l.lockFile == nil {
		return nil
	}

	hostname, _ := os.Hostname()
	info := LockInfo{
		PID:       os.Getpid(),
		StartedAt: time.Now(),
		Hostname:  hostname,
	}

	// Truncate and seek to beginning before writing
	if err := l.lockFile.Truncate(0); err != nil {
		return err
	}
	if _, err := l.lockFile.Seek(0, 0); err != nil {
		return err
	}

	return json.NewEncoder(l.lockFile).Encode(info)
}

// readLockFile reads lock info from an existing lock file.
func readLockFile(path string) (*LockInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// IsFirstInstance returns true if this is the first/primary instance.
func (l *Lock) IsFirstInstance() bool {
	return l.isFirst
}

// HolderPID returns the PID of the process holding the lock.
// Returns 0 if unknown.
func (l *Lock) HolderPID() int {
	return l.pid
}

// checkStale checks if the existing lock is stale (held by a dead process)
// and takes it over if so.
func (l *Lock) checkStale() {
	if l.isFirst || l.pid == 0 {
		return
	}

	// Check if the process holding the lock is still alive
	if !isProcessAlive(l.pid) {
		// Stale lock - take over
		os.Remove(l.path)

		file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
		if err == nil {
			l.lockFile = file
			l.isFirst = true
			l.pid = os.Getpid()
			l.writeLockInfo()
		}
	}
}

// isProcessAlive checks if a process with the given PID is still running.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, sending signal 0 checks if process exists without actually signaling
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Release releases the lock file and cleans up.
// This should be called when the application exits.
func (l *Lock) Release() {
	if l.lockFile != nil {
		l.lockFile.Close()
		l.lockFile = nil
	}

	// Only remove the lock file if we own it
	if l.isFirst {
		os.Remove(l.path)
	}

	l.isFirst = false
}

// Path returns the path to the lock file.
func (l *Lock) Path() string {
	return l.path
}
