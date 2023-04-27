#!/bin/sh

fail () {
    echo "FAILED!"
    exit 1
}

printf "check container GET endpoint: "
if docker container ls >/dev/null; then
    echo OK
else
    fail
fi

printf "check image GET endpoint: "
if docker image ls >/dev/null; then
    echo OK
else
    fail
fi

printf "check container POST endpoint: "
if ! docker container prune --force >/dev/null; then
    echo OK
else
    fail
fi

printf "check image POST endpoint: "
if ! docker image prune --force >/dev/null; then
    echo OK
else
    fail
fi

printf "check network GET endpoint: "
if ! docker network ls >/dev/null; then
    echo OK
else
    fail
fi
