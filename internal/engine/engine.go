package engine

import (
	"io"
	"os"
	"time"

	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/reader"
	"github.com/tofunmiadewuyi/dbq/internal/source"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
	"github.com/tofunmiadewuyi/dbq/utils"
)

type Engine struct {
	newDriver  func(config.DatabaseType) (source.DBDriver, error)
	newReader  func(*reader.SSHConn) (reader.FileReader, error)
	newStorage func(*storage.CloudStorage, storage.Credentials) (storage.StorageClient, error)
	zipFile    func(string, string) error
	copyFile   func(string, string) error
	openFile   func(string) (*os.File, error)
	removeFile func(string) error
	now        func() time.Time
}

func New() *Engine {
	return &Engine{
		newDriver:  source.NewDBDriver,
		newReader:  reader.GetFileReader,
		newStorage: storage.NewStorageClient,
		zipFile:    utils.ZipFile,
		copyFile:   utils.CopyFile,
		openFile:   os.Open,
		removeFile: os.Remove,
		now:        time.Now,
	}
}

func (e *Engine) sourceJob(req Request) *source.SourceJob {
	return &source.SourceJob{
		ID:       req.ID,
		Name:     req.Database.Name,
		Host:     req.Database.Host,
		Port:     req.Database.Port,
		Username: req.Database.Username,
		Password: req.Secrets.DatabasePassword,
	}
}

func (e *Engine) storageCredentials(req Request) storage.Credentials {
	return storage.Credentials{
		AccessKey: req.Secrets.StorageAccessKey,
		SecretKey: req.Secrets.StorageSecretKey,
	}
}

func closeReader(r io.Closer) {
	_ = r.Close()
}
