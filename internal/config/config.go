// Package config defines constants and types shared across the application.
package config

const AppName = "dbq"
const TmpPath = "/var/tmp"

type BinaryAnswer string

const (
	Yes BinaryAnswer = "Yes"
	No  BinaryAnswer = "No"
)

type DatabaseType string

const (
	Postgres DatabaseType = "postgres"
	MySQL    DatabaseType = "mysql"
)
