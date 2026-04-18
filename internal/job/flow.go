package job

import (
	"fmt"
	"strconv"

	"github.com/tofunmiadewuyi/dbq/utils"
	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/input"
)

func StartNewJob() error {
	return JobFlow(&Job{})
}

func EditJob(j *Job) error {
	return JobFlow(j)
}

func JobFlow(j *Job) error {
	title := "NEW JOB"
	if j.Name != "" {
		title = "EDIT JOB"
	}

	// name + id
	if j.Name == "" {
		j.Name = input.AskValid("Job name: ", func(n string) error {
			return input.ValidateField("name", n)
		}, "")
		j.ID = utils.StringToID(j.Name)
	} else {
		fmt.Printf("Name: %s\n", j.Name)
	}

	j.PrintState(title)

	// frequency
	fmt.Println("Backup frequency (cron format):")
	fmt.Println("  0 2 * * *    every day at 2am")
	fmt.Println("  0 2 * * 1    every Monday at 2am")
	fmt.Println("  0 2 1 * *    every month on the 1st at 2am")
	fmt.Println("  0 2 1 1 *    every year on Jan 1st at 2am")
	j.Frequency = input.AskValid("Enter cron: ", func(n string) error {
		return input.ValidateCron("Backup frequency", n)
	}, j.Frequency)

	j.PrintState(title)

	// database
	var db = &j.Database
	db.Type = config.DatabaseType(input.ChooseWithDefault("Database type: ", []string{string(config.Postgres), string(config.MySQL)}, string(db.Type)))

	db.Name = input.AskValid("Database name: ", func(n string) error {
		return input.ValidateField("Database name", n)
	}, db.Name)

	db.Host = input.AskValid("Database host: ", func(n string) error {
		return input.ValidateField("Database host", n)
	}, db.Host)

	if db.Port == "" {
		db.Port, _ = utils.DefaultDBPort(db.Type)
	}
	db.Port = input.AskValid("Database port: ", func(n string) error {
		return input.ValidateInt("Database port", n)
	}, db.Port)

	db.Username = input.AskValid("Database username: ", func(n string) error {
		return input.ValidateField("Database username", n)
	}, db.Username)

	db.Password = input.AskValid("Database password: ", func(n string) error {
		return input.ValidateField("Database password", n)
	}, db.Password)

	j.PrintState(title)

	// ssh
	currentSSH := string(config.No)
	if db.SSH.Required {
		currentSSH = string(config.Yes)
	}
	if config.BinaryAnswer(input.ChooseWithDefault("Connect over SSH?", []string{string(config.Yes), string(config.No)}, currentSSH)) == config.Yes {
		var ssh = &j.Database.SSH

		ssh.Host = input.AskValid("SSH Host: ", func(n string) error {
			return input.ValidateField("SSH Host", n)
		}, ssh.Host)

		sshPortDef := "22"
		if ssh.Port != 0 {
			sshPortDef = strconv.Itoa(ssh.Port)
		}
		ssh.Port = input.AskValidInt("SSH Port: ", func(n string) error {
			return input.ValidateInt("SSH Port", n)
		}, sshPortDef)

		ssh.User = input.AskValid("SSH User: ", func(n string) error {
			return input.ValidateField("SSH User", n)
		}, ssh.User)

		rawKey := input.AskValid("Path to SSH Key: ", func(n string) error {
			return input.ValidatePath("SSH Key", n)
		}, ssh.Key)
		expandedKey, err := input.ExpandPath(rawKey)
		if err != nil {
			return err
		}
		ssh.Key = expandedKey
		ssh.Required = true
		j.PrintState(title)
	} else {
		j.Database.SSH = SSHConn{}
	}

	// storage
	storageType := input.ChooseWithDefault("How will you be storing backups: ", []string{string(config.StorageCloud), string(config.StorageDirectory)}, string(j.StorageType))

	if config.StorageType(storageType) == config.StorageDirectory {
		j.StorageType = config.StorageDirectory
		j.Storage = CloudStorage{}
		j.Destination = input.AskValid("Path to directory: ", func(n string) error {
			return input.ValidateField("Destination path", n)
		}, j.Destination)
	} else {
		j.StorageType = config.StorageCloud
		j.Destination = ""

		var cloud = &j.Storage
		if cloud.Provider == "" {
			cloud.Provider = config.S3
		}
		cloud.Provider = config.StorageProvider(input.ChooseWithDefault("Storage Provider: ", []string{string(config.S3), string(config.R2)}, string(cloud.Provider)))

		if cloud.Provider == config.S3 {
			cloud.Region = input.AskValid("AWS Region: ", func(n string) error {
				return input.ValidateField("AWS Region", n)
			}, cloud.Region)
		} else {
			cloud.Endpoint = input.AskValid("R2 endpoint: ", func(n string) error {
				return input.ValidateField("R2 endpoint", n)
			}, cloud.Endpoint)
		}

		cloud.AKID = input.AskValid("Access Key ID: ", func(n string) error {
			return input.ValidateField("AKID", n)
		}, cloud.AKID)
		cloud.SAK = input.AskValid("Secret Access Key: ", func(n string) error {
			return input.ValidateField("SAK", n)
		}, cloud.SAK)
		cloud.Bucket = input.AskValid("Bucket name: ", func(n string) error {
			return input.ValidateField("Bucket name", n)
		}, cloud.Bucket)
	}

	err := j.WriteJob()
	if err != nil {
		fmt.Println("Could not save job")
	} else {
		fmt.Printf("✅ %s saved!\n", j.Name)
	}

	return err
}
