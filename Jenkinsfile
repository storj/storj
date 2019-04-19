pipeline {
    agent {
        dockerfile {
            filename 'Dockerfile.jenkins'
            args '-u root:root -v "/tmp/gomod":/go/pkg/mod'
        }
    }
    stages {
        stage('Build') {
            steps {
                checkout scm
                sh 'go mod download'

                sh 'go install -v -race ./...'
                sh 'make install-sim'

                sh 'service postgresql start'
            }
        }

        stage('Verification') {
            parallel {
                stage('Lint') {
                    steps {
                        sh 'go run ./scripts/check-copyright.go'
                        sh 'go run ./scripts/check-imports.go'
                        sh 'go run ./scripts/protobuf.go --protoc=$HOME/protoc/bin/protoc lint'
                        sh 'protolock status'
                        sh 'bash ./scripts/check-dbx-version.sh'
                        sh 'golangci-lint -j=4 run'
                        // TODO: check for go mod tidy
                        // TODO: check for directory tidy
                    }
                }

                stage('Tests') {
                    environment {
                        STORJ_POSTGRES_TEST = 'postgres://postgres@localhost/teststorj?sslmode=disable'
                    }
                    steps {
                      sh 'psql -U postgres -c \'create database teststorj;\''
                      sh 'go test -vet=off -json -race ./... | go run ./scripts/xunit.go -out tests.xml'
                      // sh 'cat test.json | tparse'
                    }

                    post {
                      always {
                        junit 'tests.xml'
                      }
                    }
                }

                stage('Integration') {
                    environment {
                        STORJ_NETWORK_HOST4 = '127.0.0.2'
                        STORJ_NETWORK_HOST6 = '127.0.0.2'
                    }
                    // cannot run in parallel, because tests may end up using ports that
                    // test-sim needs.
                    steps {
                        sh 'make test-sim'
                    }
                }
            }
        }
    }

    post {
      always {
        deleteDir()
      }
    }
}

/*
node {
  properties([disableConcurrentBuilds()])
  try {
    currentBuild.result = "SUCCESS"

    
    stage('Build') {
      steps {
        sh 'go install -race ./...'
      }
    }


    stage('Build Images') {
      sh 'make images'

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Build Binaries') {
      sh 'make binaries'

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Push Images') {
      echo 'Push to Repo'
      sh 'make push-images'
      echo "Current build result: ${currentBuild.result}"
    }

    if (env.BRANCH_NAME == "master") {
      stage('Deploy to staging') {
        sh 'make deploy'
        echo "Current build result: ${currentBuild.result}"
      }
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
*/
