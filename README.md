# Docker Db Backup

> Automatically find and backup databases running in docker containers

The script will find all running containers based on a set of rules defined in the config file. It will then attempt to find a container matching that rule, and dump it.

Exanple config file:

```yaml
config:
  # folder where the dump files will be stored
  dumpFolder: "./out"

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

  # job name
  textExact:
    dbType: postgres
    # Will match any container name exactly matching "postgres"
    matchMethod: exact
    match: "postgres"
    cron: "*/5 * * * *"
```

The script looks for the config file in the same directory it is being run from. Or in other words, the current working directory.

## Example usage

```bash
# Start test containers
(cd test && docker-compose up -d)

# Run the script
go run .
```

The script will then dump the databases to the `out` folder, and continue to run every 5 minutes because of the cron option.
