#!/usr/bin/env bash

me=$0
log_file=

teardown () {
    if [ "$log_file" ]; then
        docker compose stop
        docker compose logs --timestamps > "$log_file"
    fi
    docker compose down --volumes
}

die () {
    echo "$me: $1" >&2
    teardown
    exit 1
}

while :; do
    case $1 in
        -h|--help)
            echo "Usage: $0 [--log-file path]"
            exit 2
            ;;
        --log-file)
            if [ "$2" ]; then
                if touch "$2"; then
                    log_file=$2
                    shift
                else
                    die "log file error"
                fi
            else
                die '"--log-file" requires a non-empty option argument'
            fi
            ;;
        *)
            break
    esac
done

set -eux

# Set docker socket GID for use in docker-compose.yml
DOCKER_SOCKET_GID=$(stat -c "%g" /var/run/docker.sock)
export DOCKER_SOCKET_GID

# Start the service
docker compose up -d --build --quiet-pull docker-socket-proxy

# Wait for the service to be ready
max_attempts=5
wait_success=
for i in $(seq 1 $max_attempts); do
    if docker compose exec docker-socket-proxy /usr/local/bin/healthcheck; then
        wait_success="true"
        break
    fi
    echo "Waiting for OPC-UA proxy to be healthy: try #$i failed" >&2
    [[ $i != "$max_attempts" ]] && sleep 5
done
if [ "$wait_success" != "true" ]; then
    die "failure waiting for OPC-UA proxy to be healthy"
fi

echo "$me: run docker CLI tests"
if ! docker compose run --quiet-pull --rm docker-cli /usr/local/bin/docker-tests.sh ; then
    die "docker CLI tests failure!"
fi

echo "$me: success 🎉"
teardown
