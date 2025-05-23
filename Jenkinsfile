def lastStage = ''
node('node') {
  properties([disableConcurrentBuilds()])
  try {
    currentBuild.result = "SUCCESS"

    stage('Checkout') {
      lastStage = env.STAGE_NAME
      checkout scm

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Build Binaries') {
      lastStage = env.STAGE_NAME
      sh 'make binaries'

      //stash name: "storagenode-binaries", includes: "release/**/storagenode*.exe"

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Build Images') {
      lastStage = env.STAGE_NAME
      sh 'make images'

      echo "Current build result: ${currentBuild.result}"
    }
    
    stage('Push Images') {
      lastStage = env.STAGE_NAME
      sh 'make push-images'

      echo "Current build result: ${currentBuild.result}"
    }
  }
  catch (err) {
    echo "Caught errors! ${err}"
    echo "Setting build result to FAILURE"
    currentBuild.result = "FAILURE"

    //slackSend color: 'danger', message: "@build-team ${env.BRANCH_NAME} build failed during stage ${lastStage} ${env.BUILD_URL}"

    throw err

  }
  finally {
    stage('Cleanup') {
      sh 'make clean-images'
      deleteDir()
    }

  }
}
