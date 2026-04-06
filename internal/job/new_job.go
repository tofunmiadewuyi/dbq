package job

import (
	"fmt"

	"github.com/tofunmiadewuyi/dbq/utils"
	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/input"
)

func StartNewJob() error {

	var job Job

	// name + id
	job.Name = input.AskValid("Job name: ", func(n string) error {
		return input.ValidateField("name", n)
	}, "")
	job.ID = utils.StringToID(job.Name)

	job.PrintState()

	// frequency
	fmt.Println("Backup frequency (cron format):")
	fmt.Println("  0 2 * * *    every day at 2am")
	fmt.Println("  0 2 * * 1    every Monday at 2am")
	fmt.Println("  0 2 1 * *    every month on the 1st at 2am")
	fmt.Println("  0 2 1 1 *    every year on Jan 1st at 2am")
	job.Frequency = input.AskValid("Enter cron: ", func(n string) error {
		return input.ValidateCron("Backup frequency", n)
	}, "0 0 1 * * ")

	job.PrintState()

	// database
	var db = &job.Database
	db.Type = config.DatabaseType(input.Choose("Database type PG/MYSQL: ", []string{string(config.Postgres), string(config.MySQL)}))

	db.Name = input.AskValid("Database name: ", func(n string) error {
		return input.ValidateField("Database name", n)
	}, "")

	db.Host = input.AskValid("Database host: ", func(n string) error {
		return input.ValidateField("Database host", n)
	}, "localhost")

	defPort, _ := utils.DefaultDBPort(db.Type)
	db.Port = input.AskValid("Database port: ", func(n string) error {
		return input.ValidateInt("Database port", n)
	}, defPort)

	db.Username = input.AskValid("Database username: ", func(n string) error {
		return input.ValidateField("Database username", n)
	}, "root")

	db.Password = input.AskValid("Database password: ", func(n string) error {
		return input.ValidateField("Database password", n)
	}, "")

	job.PrintState()

	if sshRequired := input.Choose("Will we be connecting over SSH?:", []string{string(config.Yes), string(config.No)}); config.BinaryAnswer(sshRequired) == config.Yes {
		var ssh = &job.Database.SSH

		ssh.Host = input.AskValid("SSH Host: ", func(n string) error {
			return input.ValidateField("SSH Host", n)
		}, "")

		ssh.Port = input.AskValidInt("SSH Port: ", func(n string) error {
			return input.ValidateInt("SSH Port", n)
		}, "22")

		ssh.User = input.AskValid("SSH User: ", func(n string) error {
			return input.ValidateField("SSH User", n)
		}, "")

		rawKey := input.AskValid("Path to SSH Key: ", func(n string) error {
			return input.ValidatePath("SSH Key", n)
		}, "")
		expandedKey, err := input.ExpandPath(rawKey)
		if err != nil {
			return err
		}
		ssh.Key = expandedKey
		ssh.Required = true
		job.PrintState()
	}

	// storage
	if storageType := input.Choose("How will you be storing backups: ", []string{string(config.StorageCloud), string(config.StorageDirectory)}); config.StorageType(storageType) == config.StorageDirectory {
		job.StorageType = config.StorageDirectory
		job.Storage = CloudStorage{}
		job.Destination = input.AskValid("Path to directory: ", func(n string) error {
			return input.ValidateField("Destination path", n)
		}, "")
	} else {
		job.StorageType = config.StorageCloud
		job.Destination = ""

		var cloud = &job.Storage
		cloud.Provider = config.StorageProvider(input.Choose("Storage Provider: ", []string{string(config.S3), string(config.R2)}))

		if cloud.Provider == config.S3 {
			cloud.Region = input.AskValid("AWS Region: ", func(n string) error {
				return input.ValidateField("AWS Region", n)
			}, "eu-west-1")
		} else {
			// r2
			cloud.Endpoint = input.AskValid("R2 endpoint: ", func(n string) error {
				return input.ValidateField("R2 endpoint", n)
			}, "")

		}

		cloud.AKID = input.AskValid("Access Key ID: ", func(n string) error {
			return input.ValidateField("AKID", n)
		}, "")
		cloud.SAK = input.AskValid("Secret Access Key: ", func(n string) error {
			return input.ValidateField("SAK", n)
		}, "")
		cloud.Bucket = input.AskValid("Bucket name: ", func(n string) error {
			return input.ValidateField("Bucket name", n)
		}, "")

	}

	// write to file
	err := job.WriteJob()
	if err != nil {
		fmt.Println("Could not save new job")
	} else {
		fmt.Printf("%s job saved!", job.Name)
	}

	return nil
}
