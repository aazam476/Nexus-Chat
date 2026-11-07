package password

import (
	"errors"
	"fmt"
)

// Params are the Argon2id cost settings.
type Params struct {
	// MemoryKiB is the memory used per hash, in KiB.
	MemoryKiB uint32
	// Iterations is the number of passes over the memory.
	Iterations uint32
	// Parallelism is the number of lanes (threads).
	Parallelism uint8
}

// DefaultParams returns the parameters for new hashes: 64 MiB, 3 passes,
// 1 lane. This matches the RFC 9106 §4 second recommended option's memory
// with an extra pass, is above OWASP's minimum, and takes roughly a quarter
// of a second on the reference 4-vCPU host
// (docs/adr/0004-password-handling.md).
func DefaultParams() Params {
	return Params{MemoryKiB: 64 * 1024, Iterations: 3, Parallelism: 1}
}

func (params Params) String() string {
	return fmt.Sprintf("m=%d,t=%d,p=%d", params.MemoryKiB, params.Iterations, params.Parallelism)
}

// Limits on parameters accepted when verifying a stored hash. A corrupted or
// tampered hash with absurd values must not make the server allocate
// gigabytes.
const (
	maxMemoryKiB   = 1024 * 1024 // 1 GiB
	maxIterations  = 64
	maxParallelism = 16
	saltLength     = 16
	keyLength      = 32
)

var errInvalidParams = errors.New("argon2 parameters out of range")

func (params Params) validate() error {
	if params.MemoryKiB < 8*uint32(params.Parallelism) || params.MemoryKiB > maxMemoryKiB {
		return fmt.Errorf("%w: memory %d KiB", errInvalidParams, params.MemoryKiB)
	}
	if params.Iterations < 1 || params.Iterations > maxIterations {
		return fmt.Errorf("%w: %d iterations", errInvalidParams, params.Iterations)
	}
	if params.Parallelism < 1 || params.Parallelism > maxParallelism {
		return fmt.Errorf("%w: parallelism %d", errInvalidParams, params.Parallelism)
	}
	return nil
}
