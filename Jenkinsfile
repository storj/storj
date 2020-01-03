node('node') {
  properties([disableConcurrentBuilds()])
  try {
    currentBuild.result = "SUCCESS"

    stage('Checkout') {
      checkout scm

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Run Versions Test') {
        try {
          echo "Running Versions test"

          env.STORJ_SIM_POSTGRES = 'postgres://postgres@postgres:5432/teststorj?sslmode=disable'
          env.STORJ_SIM_REDIS = 'redis:6379'

          echo "STORJ_SIM_POSTGRES: $STORJ_SIM_POSTGRES"
          echo "STORJ_SIM_REDIS: $STORJ_SIM_REDIS"
          sh 'docker run --rm -d --name postgres postgres:9.6'
          sh 'docker run --rm -d --name redis redis:latest'

          sh '''until $(docker logs postgres | grep "database system is ready to accept connections" > /dev/null)
                do printf '.'
                sleep 5
                done
            '''
          sh 'docker exec postgres createdb -U postgres teststorj'
          // fetch the remote master branch
          sh 'git fetch --no-tags --progress -- https://github.com/storj/storj.git +refs/heads/master:refs/remotes/origin/master'
          sh 'docker run -u $(id -u):$(id -g) --rm -i -v $PWD:$PWD -w $PWD --entrypoint $PWD/scripts/test-sim-versions.sh -e STORJ_SIM_POSTGRES -e STORJ_SIM_REDIS --link redis:redis --link postgres:postgres -e CC=gcc storjlabs/golang:1.13.5'
        }
        catch(err){
            throw err
        }
        finally {
          sh 'docker stop postgres || true'
          sh 'docker stop redis || true'
        }
    }

    stage('Run Rolling Upgrade Test') {
        try {
          echo "Running Rolling Upgrade test"

          env.STORJ_SIM_POSTGRES = 'postgres://postgres@postgres:5432/teststorj?sslmode=disable'
          env.STORJ_SIM_REDIS = 'redis:6379'

          echo "STORJ_SIM_POSTGRES: $STORJ_SIM_POSTGRES"
          echo "STORJ_SIM_REDIS: $STORJ_SIM_REDIS"
          sh 'docker run --rm -d --name postgres postgres:9.6'
          sh 'docker run --rm -d --name redis redis:latest'

          sh '''until $(docker logs postgres | grep "database system is ready to accept connections" > /dev/null)
                do printf '.'
                sleep 5
                done
            '''
          sh 'docker exec postgres createdb -U postgres teststorj'
          // fetch the remote master branch
          sh 'git fetch --no-tags --progress -- https://github.com/storj/storj.git +refs/heads/master:refs/remotes/origin/master'
          sh 'docker run -u $(id -u):$(id -g) --rm -i -v $PWD:$PWD -w $PWD --entrypoint $PWD/scripts/tests/rollingupgrade/test-sim-rolling-upgrade.sh -e STORJ_SIM_POSTGRES -e STORJ_SIM_REDIS --link redis:redis --link postgres:postgres -e CC=gcc storjlabs/golang:1.13.5'
        }
        catch(err){
            throw err
        }
        finally {
          sh 'docker stop postgres || true'
          sh 'docker stop redis || true'
        }
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

        bat 'installer\\windows\\buildrelease.bat'

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

    slackSend color: 'danger', message: "@build-team ${env.BRANCH_NAME} build failed during stage ${env.STAGE_NAME} ${env.BUILD_URL}"

    mail from: 'builds@storj.io',
      replyTo: 'builds@storj.io',
      to: 'builds@storj.io',
      subject: "storj/storj branch ${env.BRANCH_NAME} build failed",
      body: "Project build log: ${env.BUILD_URL}"

      throw err

  }
  finally {
    stage('Cleanup') {
      sh 'make clean-images'
      deleteDir()
    }

  }
}
