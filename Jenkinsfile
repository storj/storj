node('node') {
  try {

    stage 'Checkout'

      checkout scm

    stage 'Test'

      sh """#!/bin/bash -e
        make build-dev-deps lint
      """

    stage 'Build Docker'
        echo 'Build'

    stage 'Deploy'
        echo 'Deploy'
 

    stage 'Cleanup'

      echo 'prune and cleanup'

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
}
