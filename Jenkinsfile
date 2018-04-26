node('node') {
  // Install the desired Go version
  def root = tool name: 'Go 1.10', type: 'go'

  // Export environment variables pointing to the directory where Go was installed
  withEnv(["GOROOT=${root}", "GOPATH=${root}/bin", "PATH+GO=${root}/bin"]) {
    sh 'go version'
  }

  try {

    stage('Checkout') {
      checkout scm
    }

    stage('Test') {
      sh 'export PATH=$GOPATH:$PATH && make build-dev-deps lint'
//    sh """#!/bin/bash -e
//      export PATH=$GOPATH:$PATH
//      echo $root
//      echo "path="
//      echo $PATH
//      make build-dev-deps lint
//    """
    }

    stage('Build Docker') {
        print 'Build'
    }

    stage('Deploy') {
        print 'Deploy'
    }

    stage('Cleanup') {
      print 'prune and cleanup'
    }
  }

  catch (err) {
    currentBuild.result = "FAILURE"
    throw err
  }
}
