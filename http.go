package main

import (
	"net/http"

	"github.com/rotisserie/eris"
	log "github.com/sirupsen/logrus"
)

var keyServerAddr = "serverAddr"

// handle any unknown requests
func getRoot(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not found\n"))
}

// start the http server
// handles if the http server is enabled or not
func startHttpServer(httpServer *http.Server, backup *Backup) {
	// if the http server is not defined or disabled, return
	if !k.Exists("config.httpServer.enabled") {
		return
	} else if !k.Bool("config.httpServer.enabled") {
		return
	}

	log.Infof("Starting http server")

	mux := http.NewServeMux()
	httpServer.Handler = mux

	mux.HandleFunc("/", getRoot)
	mux.HandleFunc("/api/v1/queueJob", func(w http.ResponseWriter, r *http.Request) {
		// get the job name
		jobName := r.URL.Query().Get("jobName")
		if jobName == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("jobName is required\n"))
			return
		}

		// get the job config
		jobConfig, err := getJobConfig(jobName)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error() + "\n"))
			return
		}

		// queue the job
		// should be completly synchronous
		// so when it returns, the job should be completed
		backup.QueueJob(jobConfig)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK\n"))
	})

	err := httpServer.ListenAndServe()
	if err != nil {
		if eris.Is(err, http.ErrServerClosed) {
			// ignore this error
			return
		}
		log.Fatal(eris.Wrap(err, "error in http server"))
	}
}
