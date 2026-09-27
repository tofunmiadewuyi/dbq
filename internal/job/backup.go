package job

import (
	"context"
	"fmt"

	"github.com/tofunmiadewuyi/dbq/internal/engine"
)

// CreateBackup is the CLI adapter around the execution engine. It preserves
// dbq's human output and local log format without coupling either to the core.
func CreateBackup(j *Job) error {
	result, err := engine.New().Run(context.Background(), j.EngineRequest())
	AppendLog(j.ID, "backup", result.Duration, err)
	if err != nil {
		return err
	}
	if result.StorageKey != "" {
		fmt.Printf("✅ Backup uploaded → %s\n", result.StorageKey)
	}
	if result.RetentionError != nil {
		fmt.Printf("⚠️  Backup ok, but retention cleanup failed: %v\n", result.RetentionError)
		AppendLog(j.ID, "prune", 0, result.RetentionError)
	} else if result.PrunedBackups > 0 {
		fmt.Printf("🧹 Pruned %d old backup(s), keeping latest %d\n", result.PrunedBackups, j.Retention)
	}
	fmt.Printf("✅ Backup completed in %s\n", result.Duration)
	return nil
}
