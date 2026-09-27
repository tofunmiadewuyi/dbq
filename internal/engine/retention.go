package engine

import (
	"context"
	"fmt"

	"github.com/tofunmiadewuyi/dbq/internal/storage"
)

// Prune enforces keep-last-N retention and returns the number deleted.
func (e *Engine) Prune(ctx context.Context, req Request) (int, error) {
	if req.Retention <= 0 {
		return 0, nil
	}
	switch req.StorageType {
	case storage.TypeCloud:
		client, err := e.newStorage(&req.CloudStorage, e.storageCredentials(req))
		if err != nil {
			return 0, fmt.Errorf("failed to init storage client: %w", err)
		}
		deleted, err := storage.PruneCloud(ctx, client, req.Name, req.Database.Name, req.Retention)
		return len(deleted), err
	case storage.TypeDirectory:
		deleted, err := storage.PruneDirectory(req.Destination, req.Name, req.Database.Name, req.Retention)
		return len(deleted), err
	default:
		return 0, fmt.Errorf("unknown storage type: %s", req.StorageType)
	}
}
