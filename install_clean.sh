#!/bin/bash -e

export GOPATH=$HOME/gocode
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin

rm -rf /tmp/lnd_log
go install . ./cmd/...