# Docker Db Backup

> Automatically find and backup databases running in docker containers

The script will find all running containers based on a set of rules defined in the config file. It will then attempt to find a container matching that rule, and dump it.

Exanple config file:

```yaml
config:
  # folder where the dump files will be stored
  dumpFolder: "./out"

  # optional: allows you to specify the max number of jobs to run at once
  jobLimit: 1

  # optional: allows you start an http server and trigger jobs via an api
  httpServer:
    enabled: true

jobs:
  # job name
  testRegex:
    # Database type: postgres, mysql (coming soon)
    dbType: postgres
    # Match method: regex, exact
    matchMethod: regex
    # Will match any container name starting with "test-postgres"
    match: "^test-postgres"
    # Cron schedule
    # run every 5 minutes
    cron: "*/5 * * * *"
    # optional: allows you to specify a custom db user for cmd to use
    dbUser: "postgres"

  # job name
  textExact:
    dbType: postgres
    # Will match any container name exactly matching "postgres"
    matchMethod: exact
    match: "postgres"
    cron: "*/5 * * * *"
```

> [Config.yaml](config.yaml) serves as another example used in testing if you want to see more practical usage

The script looks for the config file in the same directory it is being run from. Or in other words, the current working directory.

## Example usage

(Assuming you have the repo cloned)

```bash
# Start test containers
(cd test && docker-compose up -d)

# Run the script
go run .

# Or use the docker image
docker run --rm -it -v "$PWD:$PWD" -w "$PWD" ghcr.io/huskydog9988/docker-db-backup:latest
```

The script will then dump the databases to the `out` folder, and continue to run every 5 minutes because of the cron option.

## API

> This feature requires the `httpServer` option to be enabled in the config file

The script can also start an http server and trigger jobs via an api. This is useful if you want to trigger a job elsewhere, like from another backup service.

To trigger a job, send a GET request to `http://localhost:3333/api/v1/queueJob?jobName=JOB_NAME`.

Example using the example config file:

```bash
curl http://localhost:3333/api/v1/queueJob?jobName=testRegex
```

The server will respond with a 200 status code **after** the job has finished running. Currently, there is no way to check if the job has failed or not.
