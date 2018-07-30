node('node') {
  try {

    stage('Checkout') {
        checkout scm
    }

    stage('Build Images') {
      sh 'make test-docker images'
    }

    stage('Deploy') {
      if (env.BRANCH_NAME != 'master') {
		echo 'Skipping deploy stage'
        return
      }
      sh 'make push-images'
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
