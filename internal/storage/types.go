package storage

type StorageType string

const (
	TypeDirectory StorageType = "directory"
	TypeCloud     StorageType = "cloud"
)

type Provider string

const (
	TypeS3 Provider = "AWS (S3)"
	TypeR2 Provider = "Cloudflare R2"
)

type CloudStorage struct {
	// Access Key ID
	AKID string `toml:"access_key"`
	// Secret access key
	SAK string `toml:"secret_key"`
	// Storage Url
	Endpoint string `toml:"endpoint"`
	// Bucket name
	Bucket string `toml:"bucket"`
	// Region
	Region string `toml:"region"`
	// Provider
	Provider Provider `toml:"provider"`
}
