node('node') {
  try {
    currentBuild.result = "SUCCESS"

    stage('Checkout') {
      checkout scm

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Build Images') {
      sh 'make test-docker images'

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Push Images') {
      if (env.BRANCH_NAME == "master") {
        echo 'Push to Repo'
        sh 'make push-images'
      }

      echo "Current build result: ${currentBuild.result}"
    }

    if (env.BRANCH_NAME == "master") {
      /* This should only deploy to staging if the branch is master */
      stage('Deploy') {
        sh 'make deploy'
        for (int i = 1; i < 60; i++) {
          sh "./scripts/deploy.staging.sh storage-node-${i} storjlabs/storj-storage-node:${commit_id}"
        }
        echo "Current build result: ${currentBuild.result}"
      }
    }

  }
  catch (err) {
    echo "Caught errors! ${err}"
    echo "Setting build result to FAILURE"
    currentBuild.result = "FAILURE"

    /*
    mail body: "project build error is here: ${env.BUILD_URL}" ,
      from: 'build@storj.io',
      replyTo: 'build@storj.io',
      subject: 'project build failed',
      to: "${env.CHANGE_AUTHOR_EMAIL}"

      throw err
    */

  }
  finally {

    stage('Cleanup') {
      sh 'make test-docker-clean clean-images'
      deleteDir()
    }

  }
}
