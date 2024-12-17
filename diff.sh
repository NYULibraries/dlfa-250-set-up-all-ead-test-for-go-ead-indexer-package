#!/usr/local/bin/bash

ROOT=$( cd "$(dirname "$0")" ; pwd -P )

ACTUAL_DIR=$ROOT/tmp/actual
DIFF_DIR=$ROOT/diffs
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

if [ $? -eq 0 ]
then
    rm -fr ${DIFF_DIR:?}/*
    cd $ACTUAL_DIR
    for actualFile in $( find . -type f -name '*-add.xml' ! -name '*-commit-add.xml' )
    do
            repository=$( basename $( dirname $( dirname $actualFile ) ) )
            eadid=$( basename $( dirname $actualFile ) )
            diffDirectory=$DIFF_DIR/$repository/$eadid/
            mkdir -p $diffDirectory
            diffFile=$DIFF_DIR/$repository/$eadid/$( basename $actualFile )
            goldenFile=$GOLDEN_FILES_DIR/$repository/$eadid/$( basename $actualFile | sed 's/\.txt$/.xml/' )
            diff $goldenFile $actualFile > $diffFile
    done
fi
