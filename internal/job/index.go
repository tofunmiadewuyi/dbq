// Package job defines the job type and handles reading and writing job configurations.
package job

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/tofunmiadewuyi/dbq/utils"
	"github.com/tofunmiadewuyi/dbq/internal/config"
)

type SSHConn struct {
	Required  bool   `toml:"required"`
	Port      int    `toml:"sshport"`
	Host      string `toml:"sshhost"`
	Key       string `toml:"sshkey"`
	User      string `toml:"sshuser"`
	UseServer bool   `toml:"useserver"`
}

type DB struct {
	Name     string              `toml:"name"`
	Type     config.DatabaseType `toml:"type"`
	Port     string              `toml:"port"`
	Host     string              `toml:"host"`
	Username string              `toml:"username"`
	Password string              `toml:"password"`
	SSH      SSHConn             `toml:"ssh"`
}

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
	Provider config.StorageProvider `toml:"provider"`
}

type Job struct {
	Name        string             `toml:"name"`
	ID          string             `toml:"id"`
	StorageType config.StorageType `toml:"storage_type"`
	Destination string             `toml:"destination"` // path if directory
	Frequency   string             `toml:"frequency"`
	Database    DB                 `toml:"database"`
	Storage     CloudStorage       `toml:"storage"`
}

func (j *Job) PrintState() {
	w := 68
	box := utils.NewDisplayBox(w)
	border := box.BoxBorder()
	center := box.BoxCenter
	row := box.CreateRow

	fmt.Printf("\n┌%s┐\n", border)
	fmt.Printf("│%s│\n", center("NEW JOB"))
	fmt.Printf("├%s┤\n", border)
	row("Name:       ", j.Name)
	row("ID:         ", j.ID)
	row("Storage:    ", string(j.StorageType))
	row("Frequency:  ", utils.CronToReadable(j.Frequency))
	row("Destination:", j.Destination)

	fmt.Printf("├%s┤\n", border)
	fmt.Printf("│%s│\n", center("Database"))
	fmt.Printf("├%s┤\n", border)
	row("Type:       ", string(j.Database.Type))
	row("Host:       ", j.Database.Host)
	row("Port:       ", j.Database.Port)
	row("Name:       ", j.Database.Name)
	row("Username:   ", j.Database.Username)

	if j.Database.SSH.Required {
		fmt.Printf("├%s┤\n", border)
		fmt.Printf("│%s│\n", center("SSH"))
		fmt.Printf("├%s┤\n", border)
		row("Host:   ", j.Database.SSH.Host)
		row("Port:     ", strconv.Itoa(j.Database.SSH.Port))
		row("User:     ", j.Database.SSH.User)
		row("Key:   ", j.Database.SSH.Key)
	}

	fmt.Printf("├%s┤\n", border)
	fmt.Printf("│%s│\n", center("Storage"))
	fmt.Printf("├%s┤\n", border)
	row("Provider:   ", string(j.Storage.Provider))
	row("Bucket:     ", j.Storage.Bucket)
	if j.Storage.Provider == config.S3 {
		row("Region:     ", j.Storage.Region)
	} else {
		row("Endpoint:   ", j.Storage.Endpoint)
	}
	fmt.Printf("└%s┘\n\n", border)
}

func (j *Job) WriteJob() error {
	dir := utils.JobsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, j.ID+".toml")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(j)
}
