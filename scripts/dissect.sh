#! /bin/bash

runDissect() {
  INPUT=$1
  DIR=$2

  OUTPUT="${DIR%/}/${INPUT/.dat/.csv}"
  DIR=${OUTPUT%/*}

  mkdir -p $DIR 2> /dev/null
  if [[ $? -ne 0 ]]; then
    echo "$DIR: fail to create directory"
    return 1
  fi

  dissect $SCHEMA $INPUT > $OUTPUT
  if [[ $? -ne 0 ]]; then
    echo "unexpected error processing $INPUT"
    return 2
  fi
}

SCHEMA=$1
BASE=$2
DIR=$3

export SCHEMA
export -f runDissect

find $BASE -type f | parallel -j 4 runDissect {} $DIR
