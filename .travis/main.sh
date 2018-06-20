#!/bin/bash

# Set an option to exit immediately if any error appears
set -o errexit

# Main function that describes the behavior of the 
# script. 
# By making it a function we can place our methods
# below and have the main execution described in a
# concise way via function invocations.
main() {
  setup_dependencies
  update_docker_configuration

  echo "SUCCESS:
  Done! Finished setting up Travis machine.
  "
}

# Prepare the dependencies that the machine need.
# Here I'm just updating the apt references and then
# installing both python and python-pip. This allows
# us to make use of `pip` to fetch the latest `docker-compose`
# later.
# We also upgrade `docker-ce` so that we can get the
# latest docker version which allows us to perform
# image squashing as well as multi-stage builds.
setup_dependencies() {
  echo "INFO:
  Setting up dependencies.
  "

  sudo apt update -y
  sudo apt install realpath python python-pip -y
  sudo apt install --only-upgrade docker-ce -y

  sudo pip install docker-compose || true

  docker info
  docker-compose --version
}

# Tweak the daemon configuration so that we
# can make use of experimental features (like image
# squashing) as well as have a bigger amount of
# concurrent downloads and uploads.
update_docker_configuration() {
  echo "INFO:
  Updating docker configuration
  "

  echo '{
  "experimental": true,
  "storage-driver": "overlay2",
  "max-concurrent-downloads": 50,
  "max-concurrent-uploads": 50
}' | sudo tee /etc/docker/daemon.json
  sudo service docker restart
}

main