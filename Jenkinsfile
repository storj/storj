def lastStage = ''
node('node') {
  properties([disableConcurrentBuilds()])
  try {
    currentBuild.result = "SUCCESS"


    if (env.BRANCH_NAME == "main") {
        stage('Run UI Test') {
            lastStage = env.STAGE_NAME
            try {
              echo "Running ui test"

              env.STORJ_SIM_POSTGRES = 'postgres://postgres@postgres:5432/teststorj?sslmode=disable'
              env.STORJ_SIM_REDIS = 'redis:6379'

              echo "STORJ_SIM_POSTGRES: $STORJ_SIM_POSTGRES"
              echo "STORJ_SIM_REDIS: $STORJ_SIM_REDIS"
              sh 'docker run --rm -d -e POSTGRES_HOST_AUTH_METHOD=trust --name postgres-$BUILD_NUMBER postgres:12.3'
              sh 'docker run --rm -d --name redis-$BUILD_NUMBER redis:latest'

              sh '''until $(docker logs postgres-$BUILD_NUMBER | grep "database system is ready to accept connections" > /dev/null)
                    do printf '.'
                    sleep 5
                    done
                '''
              sh 'docker exec postgres-$BUILD_NUMBER createdb -U postgres teststorj'
              // fetch the remote master branch
              sh 'git fetch --no-tags --progress -- https://github.com/andriikotko/storj-Go-rod.git'
              sh 'go test'
            }
            catch(err){
                throw err
            }
            finally {
              sh 'docker stop postgres-$BUILD_NUMBER || true'
              sh 'docker rm postgres-$BUILD_NUMBER || true'
              sh 'docker stop redis-$BUILD_NUMBER || true'
              sh 'docker rm redis-$BUILD_NUMBER || true'
            }
        }
    }
  }

  catch (err) {
    echo "Caught errors! ${err}"
    echo "Setting build result to FAILURE"
    currentBuild.result = "FAILURE"

    mail from: 'builds@storj.io',
      replyTo: 'andrii@storj.io',
      to: 'andriis@storj.io',
      subject: "storj/storj branch ${env.BRANCH_NAME} build failed",
      body: "Project build log: ${env.BUILD_URL}"

      throw err

  }

}
