// Package privileges lets the server start as root inside its container,
// take ownership of the data directory, and then continue as an
// unprivileged user.
//
// Why: self hosters mount DATA_DIR=./data from the host. Docker creates
// that folder owned by root, so a server that starts unprivileged could not
// write its secrets there, and setup would fail on first try. Starting as
// root just long enough to fix ownership keeps the one-.env-file promise
// without running the whole server as root.
//
// It only acts when NEXUS_DROP_PRIVILEGES is set (the container image sets
// it), so tests and developer machines are never affected. Start reading at
// [Drop].
package privileges

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// EnvironmentVariable names the "uid:gid" to switch to.
const EnvironmentVariable = "NEXUS_DROP_PRIVILEGES"

// Target is the user and group to run as.
type Target struct {
	UID int
	GID int
}

// ParseTarget parses "uid:gid". An empty value means "do not drop".
func ParseTarget(value string) (Target, bool, error) {
	if value == "" {
		return Target{}, false, nil
	}
	uidText, gidText, found := strings.Cut(value, ":")
	uid, uidError := strconv.Atoi(uidText)
	gid, gidError := strconv.Atoi(gidText)
	if !found || uidError != nil || gidError != nil || uid <= 0 || gid <= 0 {
		return Target{}, false, fmt.Errorf("%s=%q must be uid:gid with both above 0", EnvironmentVariable, value)
	}
	return Target{UID: uid, GID: gid}, true, nil
}

// system is the operating-system calls Drop needs, so tests can fake them.
type system struct {
	getuid    func() int
	lchown    func(path string, uid int, gid int) error
	setgroups func(groups []int) error
	setgid    func(gid int) error
	setuid    func(uid int) error
}

// realSystem uses the real system calls. Since Go 1.16, syscall.Setuid and
// Setgid apply to every thread of the process on Linux.
func realSystem() system {
	return system{getuid: os.Getuid, lchown: os.Lchown, setgroups: syscall.Setgroups, setgid: syscall.Setgid, setuid: syscall.Setuid}
}

// Drop takes ownership of dataDirectory for target and switches the process
// to target. If the process is not root, it does nothing: the container was
// started with an explicit unprivileged user.
func Drop(dataDirectory string, target Target) error {
	return drop(realSystem(), dataDirectory, target)
}

func drop(calls system, dataDirectory string, target Target) error {
	if calls.getuid() != 0 {
		return nil
	}
	if err := os.MkdirAll(dataDirectory, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dataDirectory, err)
	}
	if err := chownTree(calls, dataDirectory, target); err != nil {
		return err
	}
	// Order matters: groups and gid must change while we are still root.
	err := errors.Join(calls.setgroups([]int{}), calls.setgid(target.GID), calls.setuid(target.UID))
	if err != nil {
		return fmt.Errorf("switching to uid %d: %w", target.UID, err)
	}
	return nil
}

// chownTree gives target every file under directory. Lchown is used so a
// symbolic link planted in the data directory cannot redirect the change
// to a file outside it.
func chownTree(calls system, directory string, target Target) error {
	err := filepath.WalkDir(directory, func(path string, _ fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		return calls.lchown(path, target.UID, target.GID)
	})
	if err != nil {
		return fmt.Errorf("taking ownership of %s: %w", directory, err)
	}
	return nil
}
