package job

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/tofunmiadewuyi/dbq/utils"
)

func GetJobs() ([]Job, error) {
    dir := utils.JobsDir()
    entries, err := os.ReadDir(dir)
    if err != nil {
        return nil, err
    }

    var jobs []Job
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
            continue
        }
        var job Job
        if _, err := toml.DecodeFile(filepath.Join(dir, entry.Name()), &job); err != nil {
            return nil, err
        }
        jobs = append(jobs, job)
    }
    return jobs, nil
}
