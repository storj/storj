node('node') {
  environment {
    GOROOT  = '${env.JENKINS_HOME}'
    GOPATH  = '$PATH:${env.JENKINS_HOME}/bin'
  }

  withEnv(["GOROOT=${env.JENKINS_HOME}/go", "PATH+GO=${env.JENKINS_HOME}/go/bin", "PATH=$PATH:${env.JENKINS_HOME}/go/bin"]) {
    sh 'go version'
  }

  try {
    sh 'echo $GOROOT'
    sh 'echo $PATH'
    sh 'echo $WORKSPACE'
    sh 'echo ENV VARS...'
    sh 'env'
    sh 'go version'

    // Install the desired Go version
    def root = tool name: 'Go 1.10', type: 'go'


    stage('Checkout') {
      checkout scm
    }

    stage('Test') {
      sh """#!/bin/bash -e
        export PATH="$PATH:$GOPATH"
        echo $root
        echo "path="
        echo $PATH
        make build-dev-deps lint
      """
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
