// Package job defines the job type and handles reading and writing job configurations.
package job

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/tofunmiadewuyi/dbq/internal/reader"
	"github.com/tofunmiadewuyi/dbq/internal/secrets"
	"github.com/tofunmiadewuyi/dbq/internal/source"
	"github.com/tofunmiadewuyi/dbq/internal/storage"
	"github.com/tofunmiadewuyi/dbq/utils"
)

var jobTemplate = template.Must(template.New("job").Parse(`# dbq job configuration
# Safe to edit: name, frequency, destination, and all database/storage connection fields.
# Do not edit: id — the scheduler uses this to identify the job.
# Credentials (password, access_key, secret_key) are stored in the system keychain.
# The empty strings below are intentional — editing them here has no effect.

name         = {{printf "%q" .Name}}
id           = {{printf "%q" .ID}}
storage_type = {{printf "%q" .StorageType}}
destination  = {{printf "%q" .Destination}}
frequency    = {{printf "%q" .Frequency}}
retention    = {{.Retention}}  # keep N most recent backups; 0 = unlimited

[database]
name     = {{printf "%q" .Database.Name}}
type     = {{printf "%q" .Database.Type}}
host     = {{printf "%q" .Database.Host}}
port     = {{printf "%q" .Database.Port}}
username = {{printf "%q" .Database.Username}}
password = ""  # stored in system keychain

[database.ssh]
required  = {{.Database.SSH.Required}}
sshhost   = {{printf "%q" .Database.SSH.Host}}
sshport   = {{.Database.SSH.Port}}
sshuser   = {{printf "%q" .Database.SSH.User}}
sshkey    = {{printf "%q" .Database.SSH.Key}}
useserver = {{.Database.SSH.UseServer}}

[storage]
provider   = {{printf "%q" .Storage.Provider}}
bucket     = {{printf "%q" .Storage.Bucket}}
region     = {{printf "%q" .Storage.Region}}
endpoint   = {{printf "%q" .Storage.Endpoint}}
access_key = ""  # stored in system keychain
secret_key = ""  # stored in system keychain
`))


// retentionLabel renders a retention count for display; 0 means keep everything.
func retentionLabel(n int) string {
	if n <= 0 {
		return "unlimited"
	}
	return fmt.Sprintf("keep last %d", n)
}

func (j *Job) PrintState(title string) {
	w := 68
	box := utils.NewDisplayBox(w)
	border := box.BoxBorder()
	center := box.BoxCenter
	row := box.CreateRow

	fmt.Printf("\n┌%s┐\n", border)
	fmt.Printf("│%s│\n", center(title))
	fmt.Printf("├%s┤\n", border)
	row("Name:       ", j.Name)
	row("ID:         ", j.ID)
	row("Storage:    ", string(j.StorageType))
	row("Frequency:  ", utils.CronToReadable(j.Frequency))
	row("Retention:  ", retentionLabel(j.Retention))
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
	if j.Storage.Provider == storage.TypeS3 {
		row("Region:     ", j.Storage.Region)
	} else {
		row("Endpoint:   ", j.Storage.Endpoint)
	}
	fmt.Printf("└%s┘\n\n", border)
}

func (j *Job) SourceJob() *source.SourceJob {
	return &source.SourceJob{
		ID:       j.ID,
		Name:     j.Database.Name,
		Host:     j.Database.Host,
		Port:     j.Database.Port,
		Username: j.Database.Username,
		Password: j.Database.Password,
	}
}

func (j *Job) ReaderSSH() *reader.SSHConn {
	return &reader.SSHConn{
		Required:  j.Database.SSH.Required,
		Port:      j.Database.SSH.Port,
		Host:      j.Database.SSH.Host,
		Key:       j.Database.SSH.Key,
		User:      j.Database.SSH.User,
		UseServer: j.Database.SSH.UseServer,
	}
}

func (j *Job) DeleteSecrets() error {
	return j.sm.Delete(j.ID)
}

func (j *Job) WriteJob() error {
	if err := storeJobSecrets(j); err != nil {
		return err
	}

	safe := *j
	safe.Database.Password = ""
	safe.Storage.AKID = ""
	safe.Storage.SAK = ""

	dir := JobsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, j.ID+".toml")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return jobTemplate.Execute(f, safe)
}

func storeJobSecrets(j *Job) error {
	if j.Database.Password != "" {
		if err := j.sm.Set(j.ID, secrets.KeyDBPassword, j.Database.Password); err != nil {
			return fmt.Errorf("failed to store db password: %w", err)
		}
	}
	if j.Storage.AKID != "" {
		if err := j.sm.Set(j.ID, secrets.KeyStorageAKID, j.Storage.AKID); err != nil {
			return fmt.Errorf("failed to store storage akid: %w", err)
		}
	}
	if j.Storage.SAK != "" {
		if err := j.sm.Set(j.ID, secrets.KeyStorageSAK, j.Storage.SAK); err != nil {
			return fmt.Errorf("failed to store storage sak: %w", err)
		}
	}
	return nil
}
