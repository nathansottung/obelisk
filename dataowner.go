package main

// dataowner.go — one Obelisk process owns a data folder at a time.
//
// Each process loads the catalog into memory and hands out record IDs from its own
// counter, so two processes on one data folder would both create record 1 and the
// later write would replace the earlier one. The database engine's locks cover one
// transaction, not the app's cached state. So the first thing a process does with a
// data folder is take an exclusive operating-system lock on a file in it, and it
// holds that lock until it has shut down. A second process is refused at once: it
// does not wait, and it does not open the folder read-write.
//
// The lock is the operating system's (LockFileEx on Windows, flock elsewhere), so
// it ends with the process, however the process ends: a crash leaves no lock behind
// and there is nothing to clean up by hand. The lock file itself stays in the folder
// and is never removed — removing it would let a second process lock a new file of
// the same name while the first still holds the old one.
//
// Within one process the lock is counted: the server takes it at start, every
// catalog open takes it again, and an operation that closes and reopens the catalog
// (a restore, an upgrade, a data folder move) holds it across the gap, so the
// folder is never unowned between the close and the reopen.
//
// Operating-system file locks are not dependable on network shares; see
// dataDirOnNetworkShare and the README.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// dataOwnerFile is the file in a data folder whose lock marks its owner.
const dataOwnerFile = "obelisk.owner"

// dataOwnerInfoAt is where the owner writes its process ID. The lock covers the
// file's first byte only, so another process can still read this part on Windows,
// where a locked byte range cannot be read.
const dataOwnerInfoAt = 64

// DataDirInUseError is the refusal a second process gets.
type DataDirInUseError struct {
	Dir string
	PID int // the owner's process ID, or 0 when it could not be read
}

func (e *DataDirInUseError) Error() string {
	who := "Another Obelisk"
	if e.PID > 0 {
		who = fmt.Sprintf("Another Obelisk (process %d)", e.PID)
	}
	return fmt.Sprintf("%s is using the data folder %s. Only one Obelisk can use a data folder at a time, "+
		"so this one did not open it and changed nothing. Use the Obelisk that is already running "+
		"(open its address in your browser), or stop it first, or start this one with -data <another folder>.",
		who, e.Dir)
}

var dataOwners = struct {
	sync.Mutex
	held map[string]*dataOwnerRef
}{held: map[string]*dataOwnerRef{}}

type dataOwnerRef struct {
	f *os.File
	n int
}

// dataOwnerKey names a data folder the same way however it was spelled.
func dataOwnerKey(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	return normPath(abs), nil
}

// acquireDataDir makes this process the owner of dir, or refuses with a
// *DataDirInUseError when another process owns it. It creates dir if needed. The
// returned release gives up this hold; the lock ends when the last hold in this
// process is given up, or when the process ends.
func acquireDataDir(dir string) (release func(), err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	key, err := dataOwnerKey(dir)
	if err != nil {
		return nil, err
	}
	dataOwners.Lock()
	defer dataOwners.Unlock()
	ref := dataOwners.held[key]
	if ref == nil {
		f, err := lockDataDir(dir)
		if err != nil {
			return nil, err
		}
		ref = &dataOwnerRef{f: f}
		dataOwners.held[key] = ref
	}
	ref.n++
	var once sync.Once
	return func() {
		once.Do(func() {
			dataOwners.Lock()
			defer dataOwners.Unlock()
			if ref.n--; ref.n == 0 {
				delete(dataOwners.held, key)
				_ = unlockFile(ref.f)
				_ = ref.f.Close()
			}
		})
	}, nil
}

// lockDataDir takes the owner lock on dir for a new holder, ignoring holds this
// process already has: a second call for the same folder is refused exactly as a
// second process would be. Tests use it to stand in for that second process.
func lockDataDir(dir string) (*os.File, error) {
	path := filepath.Join(dir, dataOwnerFile)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s to claim the data folder: %w", path, err)
	}
	if err := lockFileExclusive(f); err != nil {
		f.Close()
		return nil, &DataDirInUseError{Dir: dir, PID: readDataOwnerPID(path)}
	}
	// Who holds it, for the refusal another process will print. Best-effort.
	info := fmt.Sprintf("pid=%d\nsince=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	_, _ = f.WriteAt([]byte(fmt.Sprintf("%-64s", info)), dataOwnerInfoAt)
	return f, nil
}

// readDataOwnerPID reads the owner's process ID from its lock file, or 0.
func readDataOwnerPID(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	buf := make([]byte, 64)
	n, _ := f.ReadAt(buf, dataOwnerInfoAt)
	for _, line := range strings.Split(string(buf[:n]), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "pid="); ok {
			if pid, err := strconv.Atoi(v); err == nil && pid > 0 {
				return pid
			}
		}
	}
	return 0
}

// isDataDirInUse reports whether err is the refusal for a folder another process owns.
func isDataDirInUse(err error) bool {
	var e *DataDirInUseError
	return errors.As(err, &e)
}
