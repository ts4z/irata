#!/bin/sh -xeu

# go 1.25 has an improved garbage collector.
export GOEXPERIMENT=greenteagc

# We need this for a standalone, chroot'd deployment.
export CGO_ENABLED=0 
go build cmd/iratad/iratad.go
go build cmd/irataadmin/irataadmin.go
