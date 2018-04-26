node('node') {
  // Install the desired Go version
  def root = tool name: 'Go 1.10', type: 'go'

  // Export environment variables pointing to the directory where Go was installed
  withEnv(["GOROOT=${root}", "PATH+GO=${root}/bin"]) {
    sh 'go version'
  }

  try {

    stage('Checkout') {
      checkout scm
    }

    stage('Test') {
      sh """#!/bin/bash -e
        export PATH=$GOPATH:$PATH
        echo $root
        echo "path="
        echo $PATH
        make build-dev-deps lint
      """
    }

    stage('Build Docker') {
        echo 'Build'
    }

    stage('Deploy') {
        echo 'Deploy'
    }

    stage('Cleanup') {
      echo 'prune and cleanup'
    }
  }

  catch (err) {
    echo 'Error: ' + err
    currentBuild.result = "FAILURE"
  }
}
