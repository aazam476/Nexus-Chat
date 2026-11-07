// Package breached checks new passwords against a bundled list of
// passwords that appear in public breach dumps (spec §4.1).
//
// The list ships inside the binary, so the check works offline and never
// sends the password (or a hash of it) anywhere. See README.md for its
// source and license. Start reading at [Load].
package breached

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	_ "embed" // for the list below
)

// ErrBreached is returned for a password found in the list.
var ErrBreached = errors.New("this password appears in lists of leaked passwords; choose another")

//go:embed common-passwords.txt.gz
var compressedList []byte

// List is a sorted set of breached passwords.
type List struct {
	// passwords is sorted, so Contains can binary-search it. A sorted slice
	// uses about a quarter of the memory of a map with the same entries.
	passwords []string
}

// Load decompresses the bundled list. Call it once at startup.
func Load() (*List, error) {
	return load(compressedList)
}

func load(compressed []byte) (*List, error) {
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("opening breached-password list: %w", err)
	}
	contents, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("reading breached-password list: %w", err)
	}
	passwords := strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
	if !sort.StringsAreSorted(passwords) {
		return nil, errors.New("breached-password list is not sorted; rebuild it as README.md describes")
	}
	return &List{passwords: passwords}, nil
}

// Check returns ErrBreached if password, compared without regard to case,
// is in the list.
func (list *List) Check(password string) error {
	candidate := strings.ToLower(password)
	index := sort.SearchStrings(list.passwords, candidate)
	if index < len(list.passwords) && list.passwords[index] == candidate {
		return ErrBreached
	}
	return nil
}

// Size returns the number of passwords in the list.
func (list *List) Size() int {
	return len(list.passwords)
}
