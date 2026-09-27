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
	// Storage Url
	Endpoint string `toml:"endpoint"`
	// Bucket name
	Bucket string `toml:"bucket"`
	// Region
	Region string `toml:"region"`
	// Provider
	Provider Provider `toml:"provider"`
}

// Credentials contains cloud authentication material. It is passed at runtime
// and is never part of the persisted storage configuration.
type Credentials struct {
	AccessKey string
	SecretKey string
}
