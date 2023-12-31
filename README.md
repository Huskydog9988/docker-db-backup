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
