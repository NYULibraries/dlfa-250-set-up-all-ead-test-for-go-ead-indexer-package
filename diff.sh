#!/usr/bin/env bash

which realpath > /dev/null
if [ $? -ne 0 ]
then
    echo >&2 '`realpath` must be installed for this script to work.'
    echo >&2 'For MacOS 12.x and earlier: `brew install coreutils`'
    exit 1
fi

ROOT=$( cd "$(dirname "$0")" ; pwd -P )

LOG_DIR=$ROOT/logs

# Local clone of https://github.com/NYULibraries/findingaids_eads_v2
EAD_DIR=$1
# Local clone of https://github.com/NYULibraries/dlfa-188_v1-indexer-http-requests-xml/tree/develop/http-requests
GOLDEN_FILES_DIR=$2

time go run main.go \
    $EAD_DIR \
    $GOLDEN_FILES_DIR \
    2>$LOG_DIR/$(date +"%Y-%m-%d_%H-%M-%S")_stderr.log \
    1>$LOG_DIR/$(date +"%Y-%m-%d_%H-%M-%S").log

