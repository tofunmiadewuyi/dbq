package job

import (
	"context"
	"fmt"

	"github.com/tofunmiadewuyi/dbq/internal/engine"
)

func TestDump(j *Job) error {
	result, err := engine.New().TestSource(context.Background(), j.EngineRequest())
	AppendLog(j.ID, "test dump", result.Duration, err)
	if err != nil {
		return err
	}
	fmt.Printf("✅ Dump test passed in %s\n", result.Duration)
	return nil
}

func TestStorage(j *Job) error {
	result, err := engine.New().TestStorage(context.Background(), j.EngineRequest())
	AppendLog(j.ID, "test storage", result.Duration, err)
	if err != nil {
		return err
	}
	fmt.Printf("✅ Storage test passed in %s\n", result.Duration)
	return nil
}
