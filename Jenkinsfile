def lastStage = ''
node('node') {
  properties([disableConcurrentBuilds()])
  timeout(time: 60, unit: 'MINUTES') {
  try {
    currentBuild.result = "SUCCESS"

    stage('Checkout') {
      lastStage = env.STAGE_NAME
      checkout scm

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Postgres Run Rolling Upgrade Test') {
        // Check if changes exist in satellite/satellitedb or satellite/metabase
        def dbChanges = sh(
            script: 'git diff --name-only HEAD^ HEAD | grep -E "^satellite/(satellitedb|metabase)/" || echo "no-changes"',
            returnStdout: true
        ).trim()

        if (dbChanges == "no-changes") {
            echo "Skipping Postgres Rolling Upgrade test (no changes in satellite/satellitedb or satellite/metabase)"
            return
        }

        lastStage = env.STAGE_NAME
        echo "Running Postgres Rolling Upgrade test (database changes detected)"
        sh 'make test/rolling-upgrade/postgres'
    }

    def imageBuildType = ''
    if (env.BRANCH_NAME == 'main') {
        imageBuildType = ' -f docker-bake-main.hcl '
    }

    stage('Setup Buildx') {
      lastStage = env.STAGE_NAME
      env.DOCKER_BUILDKIT = '1'
      env.BUILDX_BUILDER = 'multiplatform-builder'
      sh 'docker buildx create --name $BUILDX_BUILDER --driver docker-container --bootstrap --use || docker buildx use $BUILDX_BUILDER'

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Build Binaries') {
      lastStage = env.STAGE_NAME

      sh 'make release/info'
      // Builds, signs, checks and compresses everything but the modular binaries.
      withCredentials([
        usernamePassword(credentialsId: 'AZURE_CODE_SIGNING', usernameVariable: 'AZURE_CLIENT_ID', passwordVariable: 'AZURE_CLIENT_SECRET'),
        string(credentialsId: 'AZURE_CODE_SIGNING_TENANT_ID', variable: 'AZURE_TENANT_ID'),
        string(credentialsId: 'AZURE_CODE_SIGNING_KEYSTORE', variable: 'SIGN_KEYSTORE'),
        string(credentialsId: 'AZURE_CODE_SIGNING_ALIAS', variable: 'SIGN_ALIAS'),
      ]) {
        sh 'make release/binaries/finalize'
      }

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Build Images') {
      lastStage = env.STAGE_NAME
      sh 'make release/images/build'

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Push Images') {
      lastStage = env.STAGE_NAME
      sh 'make release/images/push'

      echo "Current build result: ${currentBuild.result}"
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

    stage('Build Modular Binaries') {
      lastStage = env.STAGE_NAME
      sh 'make release/binaries/build-modular-storagenode'
      sh 'make release/binaries/build-modular-satellite'

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Compress Modular Binaries') {
      lastStage = env.STAGE_NAME

      sh 'make release/binaries/compress'

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Publish Release To Github') {
      withCredentials([string(credentialsId: 'GITHUB_RELEASE_TOKEN', variable: 'GITHUB_TOKEN')]) {
        lastStage = env.STAGE_NAME
        sh 'make release/binaries/publish-to-github'

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
      sh 'make release/images/clean'
      sh '[ -n "$BUILDX_BUILDER" ] && docker buildx rm --keep-state $BUILDX_BUILDER || true'
      deleteDir()
    }
  }
  }
}
