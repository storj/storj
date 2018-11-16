#!/bin/bash
set -ueo pipefail

rm -rf ~/.storj/capt
go install ./...
captplanet setup
captplanet run