node('node') {
  try {

    stage('Checkout') {
        checkout scm
    }

    stage('Build Images') {
      sh 'make test-docker images'
    }

    stage('Push Images') {
      echo 'Push to Repo'
      sh 'make push-images'
    }

    stage('Deploy') {
      /* This should only deploy to staging if the branch is master */
      if (env.BRANCH_NAME == "master") {
        sh "./scripts/deploy.staging.sh satellite storjlabs/storj-satellite:${commit_id}"
        for (int i = 1; i < 60; i++) {
          sh "./scripts/deploy.staging.sh storage-node-${i} storjlabs/storj-storage-node:${commit_id}"
        }
      }

      return
    }

  }
  catch (err) {
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
