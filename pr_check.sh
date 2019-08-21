# Set the `GOBIN` environment variable so that dependencies will be installed
# always in the same place, regardless of the value of `GOPATH`:
export GOBIN="${PWD}/.gobin"
export PATH="${GOBIN}:${PATH}"

make test build
