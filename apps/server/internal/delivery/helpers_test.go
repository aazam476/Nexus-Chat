package delivery

import (
	"context"
	"fmt"
	"strings"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
)

// conflictOnce answers the first commit as if another one had won.
type conflictOnce struct {
	deliveryStore
	done bool
}

func (fake *conflictOnce) Commit(ctx context.Context, params store.CommitParams) (int64, error) {
	if !fake.done {
		fake.done = true
		return 0, store.ErrEpochConflict
	}
	return fake.deliveryStore.Commit(ctx, params)
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func second[T any](_ T, err error) error {
	return err
}

func typeName(value any) string {
	name := fmt.Sprintf("%T", value)
	return name[strings.LastIndex(name, ".")+1:]
}
