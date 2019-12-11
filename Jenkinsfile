node('node') {
  properties([disableConcurrentBuilds()])
  try {
    currentBuild.result = "SUCCESS"

    stage('Checkout') {
      checkout scm

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Run Versions Test') {
      echo "Running migration test"

      environment {
        STORJ_SIM_POSTGRES = 'postgres://postgres@localhost:58723/teststorj?sslmode=disable'
      }

      sh 'docker run --rm -p 58723:5432 -d --name postgres postgres'
      sh 'psql -U postgres -p 58723 -c \'create database teststorj;\''
      sh './scripts/test-sim-versions.sh'
      sh 'docker rm -f postgres'
    }

    stage('Build Binaries') {
      sh 'make binaries'

      stash name: "storagenode-binaries", includes: "release/**/storagenode*.exe"

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Build Windows Installer') {
      node('windows') {
        checkout scm

        unstash "storagenode-binaries"

        bat 'installer\\windows\\build.bat'

        stash name: "storagenode-installer", includes: "release/**/storagenode*.msi"

        echo "Current build result: ${currentBuild.result}"
      }
    }

    stage('Sign Windows Installer') {
      unstash "storagenode-installer"

      sh 'make sign-windows-installer'

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Build Images') {
      sh 'make images'

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Push Images') {
      echo 'Push to Repo'
      sh 'make push-images'
      echo "Current build result: ${currentBuild.result}"
    }

    stage('Upload') {
      sh 'make binaries-upload'
      echo "Current build result: ${currentBuild.result}"
    }

  }
  catch (err) {
    echo "Caught errors! ${err}"
    echo "Setting build result to FAILURE"
    currentBuild.result = "FAILURE"

    /*
    slackSend color: 'danger', message: "@channel ${env.BRANCH_NAME} build failed during stage ${env.STAGE_NAME} ${env.BUILD_URL}"

    mail from: 'builds@storj.io',
      replyTo: 'builds@storj.io',
      to: 'builds@storj.io',
      subject: "storj/storj branch ${env.BRANCH_NAME} build failed",
      body: "Project build log: ${env.BUILD_URL}"
    */

      throw err

  }
  finally {

    stage('Cleanup') {
      sh 'make clean-images'
      deleteDir()
    }

  }
}
