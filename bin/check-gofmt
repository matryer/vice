#! /bin/bash

set -e

files=$(gofmt -l .)
if [[ $files ]]; then
  echo "The following files need formatting"
  echo "$files"
  exit 1
fi
