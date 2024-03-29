package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/go-co-op/gocron/v2"
	"github.com/rotisserie/eris"
	log "github.com/sirupsen/logrus"
)

// array of job names
var jobs []string

type JobConfig struct {
	Name   string
	Config map[string]string
}

func init() {
	// Log as JSON instead of the default ASCII formatter.
	// log.SetFormatter(&log.JSONFormatter{})
	log.SetFormatter(&log.TextFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// // Only log the warning severity or above.
	// log.SetLevel(log.WarnLevel)
	log.SetLevel(log.DebugLevel)
}

func main() {
	log.Info("Starting backup service")

	// load config file
	loadConfigFile()

	// create backup folder
	createBackupFolder()

	// create a scheduler
	s, err := gocron.NewScheduler()
	if err != nil {
		log.Fatal(eris.Wrap(err, "failed to create scheduler"))
	}
	defer func() { _ = s.Shutdown() }()

	// create docker client
	log.Debug("Creating docker client")
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(eris.Wrap(err, "failed to create docker client"))
	}
	defer cli.Close()

	backup := NewBackup(cli)

	// get a list of every job
	jobs = k.MapKeys("jobs")

	// start the http server
	// also handles if it is disabled
	httpServer := &http.Server{
		Addr: ":3333",
	}
	go startHttpServer(httpServer, backup)
	defer func() {
		err := httpServer.Shutdown(context.Background())
		if err != nil {
			log.Error(eris.Wrap(err, "failed to shutdown http server"))
		}
	}()

	// schedule each job
	for _, job := range jobs {
		log.Infof("Scheduling job: %s", job)

		jobConfig, err := getJobConfig(job)
		if err != nil {
			log.Error(eris.Wrapf(err, "failed to get job config for %s", job))
		}

		_, err = s.NewJob(gocron.CronJob(jobConfig.Config["cron"], false), gocron.NewTask(backup.QueueJob, jobConfig))
		if err != nil {
			log.Error(eris.Wrapf(err, "failed to schedule job %s", jobConfig.Name))
		}
	}

	// start the scheduler
	s.Start()

	// wait for a signal to end the program
	endSignal := make(chan os.Signal, 1)
	signal.Notify(endSignal, syscall.SIGINT, syscall.SIGTERM)
	<-endSignal
}

func (b Backup) QueueJob(jobConfig *JobConfig) {
	ctx := context.Background()
	log.Debug("Listing containers")
	// gets a list of running containers
	containers, err := b.Cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		log.Error(eris.Wrap(err, "failed to list containers"))
		return
	}

	targetIds := []string{}
	for _, container := range containers {
		if isTargetContainer(container, jobConfig) {
			targetIds = append(targetIds, container.ID)
		}
	}

	log.Infof("Found %d target container(s) for job %s", len(targetIds), jobConfig.Name)
	for _, target := range targetIds {
		log.Infof("Queueing backup for container %s", target)
		// tell queue an item entered
		b.backupQueue <- struct{}{}
		b.backupContainer(ctx, &BackupContainerOptions{
			ContainerId: target,
			JobConfig:   jobConfig,
		})
	}
}

// test if a container is the target container
// based on the config for the job
func isTargetContainer(container types.Container, jobConfig *JobConfig) bool {
	// check if the container is running
	if container.State != "running" {
		log.Debugf("Container %s is not running", container.ID)
		return false
	}

	if jobConfig.Config["matchMethod"] == "exact" {
		for _, name := range container.Names {
			// clean up the container name
			name = preprocesContainerName(name)

			// check if the container has the target name
			if name != jobConfig.Config["match"] {
				log.Debugf("Container %s (%s) is not the target name of %s", name, container.ID, jobConfig.Config["match"])
				return false
			} else {
				log.Debugf("Container %s (%s) is the target name of %s", name, container.ID, jobConfig.Config["match"])
			}
		}
	}

	if jobConfig.Config["matchMethod"] == "regex" {
		// check if the container matches the regex

		re, err := regexp2.Compile(jobConfig.Config["match"], 0)
		if err != nil {
			log.Fatal(eris.Wrapf(err, "failed to compile regex for job %s", jobConfig.Name))
		}

		for _, name := range container.Names {
			// clean up the container name
			name = preprocesContainerName(name)

			// test with regex
			isMatch, err := re.MatchString(name)
			if err != nil {
				log.Fatal(eris.Wrapf(err, "failed to match regex for job %s", jobConfig.Name))
			}

			// if the regex doesn't match, return false
			if !isMatch {
				log.Debugf("Container %s (%s) is not the target regex for job %s", name, container.ID, jobConfig.Name)
				return false
			} else {
				log.Debugf("Container %s (%s) is the target regex for job %s", name, container.ID, jobConfig.Name)
			}
		}

	}

	return true
}

func preprocesContainerName(name string) string {
	// remove the leading slash on container names
	_, i := utf8.DecodeRuneInString(name)
	return name[i:]
}

func getJobConfig(name string) (*JobConfig, error) {
	if !k.Exists("jobs." + name) {
		return nil, eris.Errorf("Job %s does not exist", name)
	}

	return &JobConfig{
		Name:   name,
		Config: k.StringMap("jobs." + name),
	}, nil
}
