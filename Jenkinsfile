node('node') {
  properties([disableConcurrentBuilds()])
  try {
    currentBuild.result = "SUCCESS"

    stage('Checkout') {
      checkout scm

      echo "Current build result: ${currentBuild.result}"
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

    stage('Test Windows Installer') {
      node('windows') {
        checkout scm

//         unstash "storagenode-installer"

        // NB: using environment variables like this only works
        //     for non-concurrent builds. For concurrent builds
        //     use `set ... && call <batch file>`:

        // Set scheduled tasks log path
        bat 'setx scheduledTaskLog %TEMP%\\scheduledTask.log'

        // Store msiPath in environment variable
        bat 'for /d %%d in (release\\*) do setx msiPath %%d\\storagenode_windows_amd64.msi'
        // Task reads msiPath from environment variable
        bat 'schtasks /run /tn "CI Installer Test Elevated"'
        // TODO: remove
        bat 'cmd /c type %%scheduledTaskLog%%'
        // Print output and check for non-zero status
        bat 'cmd /c go run ./scripts/parse-scheduled-task-output.go %%scheduledTaskLog%%'

        // Cleanup
        bat 'cmd /c del %%scheduledTaskLog%%'

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

    if (env.BRANCH_NAME == 'master') {
      slackSend color: 'danger', message: "@channel ${env.BRANCH_NAME} build failed during stage ${env.STAGE_NAME} ${env.BUILD_URL}"

      mail from: 'builds@storj.io',
        replyTo: 'builds@storj.io',
        to: 'builds@storj.io',
        subject: "storj/storj branch ${env.BRANCH_NAME} build failed",
        body: "Project build log: ${env.BUILD_URL}"
    }

    throw err

  }
  finally {

    stage('Cleanup') {
      sh 'make clean-images'
      deleteDir()
    }

  }
}
