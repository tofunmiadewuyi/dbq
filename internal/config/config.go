// Package config defines constants and types shared across the application.
package config

const AppName = "dbq"
const TmpPath = "/var/tmp"

type DatabaseType string
const (
	Postgres DatabaseType = "postgres"
	MySQL    DatabaseType = "mysql"
)

type StorageType string
const (
	StorageDirectory StorageType = "directory"
	StorageCloud     StorageType = "cloud"
)

type StorageProvider string
const (
	S3 StorageProvider = "AWS (S3)"
	R2 StorageProvider = "Cloudflare R2"
)

type BinaryAnswer string
const (
	Yes BinaryAnswer = "Yes"
	No BinaryAnswer = "No"
)

