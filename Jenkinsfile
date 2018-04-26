node('node') {
  /*
  environment {
    GOROOT  = '${env.JENKINS_HOME}'
    GOPATH  = '$PATH:${env.JENKINS_HOME}/bin'
  }
  */

  // Install the desired Go version
  def root = tool name: 'Go 1.10', type: 'go'

  try {
    withEnv(["GOROOT=${root}", "PATH+GO=${root}/bin", "GOPATH=${root}/bin"]) {
      stage('Checkout') {
        checkout scm
      }

      stage('Test') {
        sh 'echo $GOROOT'
        sh 'echo $GOPATH'
        sh 'echo $PATH'
        sh 'make build-dev-deps lint'
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
  }

  catch (err) {
    currentBuild.result = "FAILURE"
    throw err
  }
}
