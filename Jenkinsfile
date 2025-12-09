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

    stage('CRDB Run Rolling Upgrade Test') {
        // Check if changes exist in satellite/satellitedb or satellite/metabase
        def dbChanges = sh(
            script: 'git diff --name-only HEAD^ HEAD | grep -E "^satellite/(satellitedb|metabase)/" || echo "no-changes"',
            returnStdout: true
        ).trim()

        if (dbChanges == "no-changes") {
            echo "Skipping CRDB Rolling Upgrade test (no changes in satellite/satellitedb or satellite/metabase)"
            return
        }

        lastStage = env.STAGE_NAME
        try {
          echo "Running CRDB Rolling Upgrade test (database changes detected)"

          env.STORJ_SIM_POSTGRES='cockroach://root@cockroach:26257/master?sslmode=disable'
          env.STORJ_SIM_REDIS='redis:6379'
          env.STORJ_MIGRATION_DB='cockroach://root@cockroach:26257/master?sslmode=disable'
          env.STORJ_SKIP_FIX_LAST_NETS=true
          env.STORJ_CONSOLE_SIGNUP_ACTIVATION_CODE_ENABLED = "false"

          echo "STORJ_SIM_POSTGRES: $STORJ_SIM_POSTGRES"
          echo "STORJ_SIM_REDIS: $STORJ_SIM_REDIS"
          echo "STORJ_MIGRATION_DB: $STORJ_MIGRATION_DB"
          sh 'docker run --rm -d --name cockroach-$BUILD_NUMBER cockroachdb/cockroach:v23.2.2 start-single-node --insecure'
          sh 'docker run --rm -d --name redis-$BUILD_NUMBER redis:latest'
          sleep 1
          sh '''until $(docker exec cockroach-$BUILD_NUMBER cockroach sql --insecure -e "select * from now();" > /dev/null)
                do printf '.'
                sleep 1
                done
            '''

          // fetch the remote main branch
          sh 'git fetch --no-tags --progress -- https://github.com/storj/storj.git +refs/heads/main:refs/remotes/origin/main'
          sh 'docker run -u $(id -u):$(id -g) --rm -i -v $PWD:$PWD -w $PWD --entrypoint $PWD/testsuite/rolling-upgrade/start-sim.sh -e BRANCH_NAME -e STORJ_SIM_POSTGRES -e STORJ_SIM_REDIS -e STORJ_MIGRATION_DB -e STORJ_SKIP_FIX_LAST_NETS -e STORJ_CONSOLE_SIGNUP_ACTIVATION_CODE_ENABLED --link redis-$BUILD_NUMBER:redis --link cockroach-$BUILD_NUMBER:cockroach storjlabs/golang:1.24.7'
        }
        catch(err){
            throw err
        }
        finally {
          sh 'docker stop cockroach-$BUILD_NUMBER || true'
          sh 'docker rm cockroach-$BUILD_NUMBER || true'
          sh 'docker stop redis-$BUILD_NUMBER || true'
          sh 'docker rm redis-$BUILD_NUMBER || true'
        }
    }

    stage('Build Binaries') {
      lastStage = env.STAGE_NAME
      sh 'make binaries'

      stash name: "storagenode-binaries", includes: "release/**/storagenode*.exe"

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Build Darwin Binaries') {
      lastStage = env.STAGE_NAME
      sh 'make darwin-binaries'

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

    def imageBuildType = ''
    if (env.BRANCH_NAME == 'main') {
        imageBuildType = ' -f docker-bake-main.hcl '
    }

    stage('Publish Modular Satellite Images') {
          lastStage = env.STAGE_NAME
          env.MODULE="SATELLITE"
          sh './scripts/bake.sh -f docker-bake.hcl ' + imageBuildType + ' satellite-modular --push'
    }

    stage('Publish Modular Storagenode Images') {
          lastStage = env.STAGE_NAME
          env.MODULE="STORAGENODE"
          sh './scripts/bake.sh -f docker-bake.hcl ' + imageBuildType + ' storagenode-modular --push'
    }

    stage('Build Windows Installer') {
      lastStage = env.STAGE_NAME
      node('windows') {
        checkout scm

        unstash "storagenode-binaries"

        bat 'installer\\windows\\buildrelease.bat'

        stash name: "storagenode-installer", includes: "release/**/storagenode*.msi"

        echo "Current build result: ${currentBuild.result}"
      }
    }

    stage('Sign Windows Installer') {
      lastStage = env.STAGE_NAME
      unstash "storagenode-installer"

      sh 'make sign-windows-installer'

      echo "Current build result: ${currentBuild.result}"
    }


    stage('Upload') {
      lastStage = env.STAGE_NAME
      sh 'make binaries-upload'

      echo "Current build result: ${currentBuild.result}"
    }
    stage('Publish Release') {
      withCredentials([string(credentialsId: 'GITHUB_RELEASE_TOKEN', variable: 'GITHUB_TOKEN')]) {
        lastStage = env.STAGE_NAME
        sh 'make publish-release'

        echo "Current build result: ${currentBuild.result}"
      }
    }

  }
  catch (err) {
    echo "Caught errors! ${err}"
    echo "Setting build result to FAILURE"
    currentBuild.result = "FAILURE"

    slackSend color: 'danger', message: "@build-team ${env.BRANCH_NAME} build failed during stage ${lastStage} ${env.BUILD_URL}"

    throw err

  }
  finally {
    stage('Cleanup') {
      sh 'make clean-images'
      deleteDir()
    }

  }
}
