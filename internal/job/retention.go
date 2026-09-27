package job

import (
	"context"

	"github.com/tofunmiadewuyi/dbq/internal/engine"
)

// PruneBackups is the CLI-facing adapter for an explicit prune operation.
func PruneBackups(j *Job) (int, error) {
	return engine.New().Prune(context.Background(), j.EngineRequest())
}
