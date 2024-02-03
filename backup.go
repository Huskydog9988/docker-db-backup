package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/rotisserie/eris"
	log "github.com/sirupsen/logrus"
)

type Backup struct {
	// docker client
	Cli         *client.Client
	backupQueue chan struct{}
}

func NewBackup(cli *client.Client) *Backup {
	return &Backup{
		Cli:         cli,
		backupQueue: make(chan struct{}, getJobLimit()),
	}
}

type BackupContainerOptions struct {
	// container id to backup
	ContainerId string
	// config for this job
	JobConfig *JobConfig
}

func (b Backup) backupContainer(ctx context.Context, options *BackupContainerOptions) {
	log.Infof("Backing up container %s", options.ContainerId)
	// tell queue an item left after we're done
	defer func() {
		<-b.backupQueue
		log.Debugf("Removed %s from queue", options.ContainerId)
	}()

	backupCmd, dbPassword := getBackupCommand(options.JobConfig)

	// create exec for container
	log.Debugf("Creating exec for container %s", options.ContainerId)
	execIDRes, err := b.Cli.ContainerExecCreate(ctx, options.ContainerId, types.ExecConfig{
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          backupCmd,

		// if we have a password, we need to attach stdin
		AttachStdin: dbPassword != "",
	})
	if err != nil {
		log.Error(eris.Wrap(err, "failed to create exec for docker container"))
		return
	}

	// attach to exec
	log.Debugf("Attaching to exec for container %s", options.ContainerId)
	highjackRes, err := b.Cli.ContainerExecAttach(ctx, execIDRes.ID, types.ExecStartCheck{})
	if err != nil {
		log.Error(eris.Wrap(err, "failed to attach to exec for docker container"))
		return
	}
	defer highjackRes.Close()

	targetContainer, err := b.Cli.ContainerInspect(ctx, options.ContainerId)
	if err != nil {
		log.Error(eris.Wrap(err, "failed to inspect container"))
		return
	}

	// create dump file
	log.Debug("Creating dump file")
	dumpFile, err := os.Create(getBackupFileName(options.JobConfig, preprocesContainerName(targetContainer.Name)))
	if err != nil {
		log.Error(eris.Wrap(err, "failed to create file"))
		return
	}
	defer dumpFile.Close()

	// read the output
	stdOutReader, stdOutWriter := io.Pipe()
	defer stdOutWriter.Close()
	defer stdOutReader.Close()

	var errBuf bytes.Buffer
	outputDone := make(chan error)

	// write dump to file
	go func() {
		log.Debug("Writing dump to file")

		io.Copy(dumpFile, stdOutReader)
	}()

	go func() {
		log.Debug("Reading output from exec")
		// StdCopy demultiplexes the stream into two buffers
		_, err = stdcopy.StdCopy(stdOutWriter, &errBuf, highjackRes.Reader)
		outputDone <- err
	}()

	if dbPassword != "" {
		// write password to stdin
		highjackRes.Conn.Write([]byte(dbPassword + "\n"))
	}

	select {
	case err := <-outputDone:
		if err != nil {
			// stdOutReader.Close()
			log.Error(eris.Wrap(err, "failed to read output from exec"))
		}
		log.Debug("Finished reading output from exec")
		// stdOutReader.Close()
		break

	case <-ctx.Done():
		if ctx.Err() != nil {
			log.Error(eris.Wrap(err, "context cancelled"))
			return
		}
	}

	// stdout, err := io.ReadAll(&outBuf)
	// if err != nil {
	// 	log.Fatal(eris.Wrap(err, "failed to read stdout on exec"))
	// }

	res, err := b.Cli.ContainerExecInspect(ctx, execIDRes.ID)
	if err != nil {
		if err != nil {
			log.Error(eris.Wrap(err, "failed to inspect exec"))
			return
		}
	}

	// log.Info(string(stdout))
	if res.ExitCode != 0 {
		// dumpFile.

		// read error stderr
		stderr, err := io.ReadAll(&errBuf)
		if err != nil {
			log.Error(eris.Wrap(err, "failed to read stderr on exec"))
		}
		log.Error(string(stderr))

		log.Errorf("Failed to backup %s, exec failed with exit code %d", options.ContainerId, res.ExitCode)
		return
	}

	log.Infof("Finished backing up container %s", options.ContainerId)
}

// The backup command to run in the container
// Also returns the password for the database if needed
func getBackupCommand(jobConfig *JobConfig) ([]string, string) {
	if jobConfig.Config["dbType"] == "postgres" {

		cmd := []string{"pg_dump"}

		// default to postgres user
		pgUser := "postgres"

		// if we have a custom user, use it
		if jobConfig.Config["dbUser"] != "" {

			log.Debugf("Using custom postgres user: %s", jobConfig.Config["dbUser"])
			pgUser = jobConfig.Config["dbUser"]
		} else {
			log.Debugf("Using default postgres user: %s", jobConfig.Config["dbUser"])
		}
		// add the user to the command
		cmd = append(cmd, "-U", pgUser)

		// if we have a password, we need add the password flag
		if jobConfig.Config["dbPassword"] != "" {
			cmd = append(cmd, "--password")
		}

		// add additional args to the command
		if jobConfig.Config["dbAdditionalArgs"] != "" {
			cmd = append(cmd, jobConfig.Config["dbAdditionalArgs"])
		}

		// https://www.postgresql.org/docs/current/app-pg-dumpall.html
		return cmd, jobConfig.Config["dbPassword"]
	}

	// if jobConfig.Config["dbType"] == "mariadb" {
	// 	cmd := []string{"mariadb-dump"}

	// 	return cmd, jobConfig.Config["dbPassword"]
	// }

	return []string{"echo", "Unknown db type"}, ""
}

// generate the backup file name
func getBackupFileName(jobConfig *JobConfig, containerName string) string {
	// currentTime := time.Now()

	// currentTime.Format("2006-01-02T15_04_05")

	// return fmt.Sprintf("dump_%s_%s_%s.out", jobConfig.Name, containerName, currentTime.Format("2006-01-02T15_04_05"))

	return fmt.Sprintf("%s/%s.dump", k.String("config.dumpFolder"), containerName)
}

// create the folder to store the backups in
func createBackupFolder() {
	// ensure key exists
	if !k.Exists("config.dumpFolder") {
		log.Info("No dump folder specified, using default")
		k.Set("config.dumpFolder", "./out")
	}
	log.Infof("Setting dump folder to %s", k.String("config.dumpFolder"))

	err := os.MkdirAll(k.String("config.dumpFolder"), os.ModePerm)
	if err != nil {
		log.Panic(eris.Wrap(err, "failed to create dump folder"))
	}
}

func getJobLimit() int {
	if k.Exists("config.jobLimit") {
		return k.Int("config.jobLimit")
	}

	return 1
}
