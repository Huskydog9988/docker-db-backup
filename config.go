package main

import (
	"os"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/rotisserie/eris"
	log "github.com/sirupsen/logrus"
)

// Global koanf instance. Use "." as the key path delimiter. This can be "/" or any character.
var k = koanf.New(".")

// loadConfigFile loads the config file from the current directory and /etc/docker-db-backup/config.yaml
func loadConfigFile() {
	configPath := "config.yaml"

	if checkIfFileExists("config.yaml") {
		// do nothing
	} else if checkIfFileExists("/etc/docker-db-backup/config.yaml") {
		configPath = "/etc/docker-db-backup/config.yaml"
	} else {
		log.Fatal("Config file not found")
	}

	log.Infof("Loading config file: %s", configPath)

	// Load yaml config.
	if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
		log.Fatal(eris.Wrap(err, "failed to load config"))
	}
}

// utility function to check if a file exists
func checkIfFileExists(path string) bool {
	// based on
	// https://stackoverflow.com/questions/12518876/how-to-check-if-a-file-exists-in-go
	if _, err := os.Stat(path); err == nil {
		// path/to/whatever exists
		return true
	} else if eris.Is(err, os.ErrNotExist) {
		// path/to/whatever does *not* exist
		return false
	} else {
		// Schrodinger: file may or may not exist. See err for details.

		// Therefore, do *NOT* use !os.IsNotExist(err) to test for file existence
		return false
	}

	// return false
}
