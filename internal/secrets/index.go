// Package secrets provides an interface for storing and retrieving job credentials
// from the OS keychain, keeping sensitive values off disk.
package secrets

const (
	KeyDBPassword  = "db_password"
	KeyStorageAKID = "storage_akid"
	KeyStorageSAK  = "storage_sak"
)

// Manager stores and retrieves credentials for a job.
type Manager interface {
	Set(jobID, key, value string) error
	Get(jobID, key string) (string, error)
	Delete(jobID string) error
}

func New() Manager {
	return &keyringManager{}
}
