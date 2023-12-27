package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/rotisserie/eris"
	log "github.com/sirupsen/logrus"
)

type BackupContainerOptions struct {
	// container id to backup
	ContainerId string
	// config for this job
	JobConfig *JobConfig
	// docker client
	Cli *client.Client
}

func backupContainer(ctx context.Context, options *BackupContainerOptions) {
	log.Infof("Backing up container %s", options.ContainerId)

	// create exec for container
	log.Debugf("Creating exec for container %s", options.ContainerId)
	execIDRes, err := options.Cli.ContainerExecCreate(ctx, options.ContainerId, types.ExecConfig{
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          getBackupCommand(options.JobConfig),
	})
	if err != nil {
		log.Fatal(eris.Wrap(err, "failed to create exec for docker container"))
	}

	// attach to exec
	log.Debugf("Attaching to exec for container %s", options.ContainerId)
	highjackRes, err := options.Cli.ContainerExecAttach(ctx, execIDRes.ID, types.ExecStartCheck{})
	if err != nil {
		log.Fatal(eris.Wrap(err, "failed to attach to exec for docker container"))
	}
	defer highjackRes.Close()

	targetContainer, err := options.Cli.ContainerInspect(ctx, options.ContainerId)
	if err != nil {
		log.Fatal(eris.Wrap(err, "failed to inspect container"))
	}

	// create dump file
	log.Debug("Creating dump file")
	dumpFile, err := os.Create(getBackupFileName(options.JobConfig, preprocesContainerName(targetContainer.Name)))
	if err != nil {
		log.Panic(eris.Wrap(err, "failed to create file"))
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

	select {
	case err := <-outputDone:
		if err != nil {
			// stdOutReader.Close()
			log.Fatal(eris.Wrap(err, "failed to read output from exec"))
		}
		// log.Debug("Finished reading output from exec")
		// stdOutReader.Close()
		break

	case <-ctx.Done():
		if ctx.Err() != nil {
			log.Fatal(eris.Wrap(err, "context cancelled"))
		}
	}

	// stdout, err := io.ReadAll(&outBuf)
	// if err != nil {
	// 	log.Fatal(eris.Wrap(err, "failed to read stdout on exec"))
	// }

	res, err := options.Cli.ContainerExecInspect(ctx, execIDRes.ID)
	if err != nil {
		if err != nil {
			log.Fatal(eris.Wrap(err, "failed to inspect exec"))
		}
	}

	// log.Info(string(stdout))
	if res.ExitCode != 0 {
		// dumpFile.

		// read error stderr
		stderr, err := io.ReadAll(&errBuf)
		if err != nil {
			log.Fatal(eris.Wrap(err, "failed to read stderr on exec"))
		}
		log.Info(string(stderr))

		log.Fatalf("Failed to backup %s, exec failed with exit code %d", options.ContainerId, res.ExitCode)
	}
}

func getBackupCommand(jobConfig *JobConfig) []string {
	if jobConfig.Config["dbType"] == "postgres" {
		return []string{"pg_dumpall", "-c", "-w", "-U", "postgres"}
	}

	// log.Info("pg_dumpall -c -U postgres > dump_`date +%Y-%m-%d\"_\"%H_%M_%S`.sql")

	return []string{"echo", "hello world"}
}

func getBackupFileName(jobConfig *JobConfig, containerName string) string {
	currentTime := time.Now()

	currentTime.Format("2006-01-02T15_04_05")

	return fmt.Sprintf("dump_%s_%s_%s.out", jobConfig.Name, containerName, currentTime.Format("2006-01-02T15_04_05"))
}
