pipeline {
    agent {
        docker {
            image 'golang:1.12'
            args '-u root:root -v "/tmp/gomod":/go/pkg/mod'
        }
    }
    stages {
        stage('Environment') {
            steps {
                sh 'apt-get update && apt-get install unzip -y'
                sh 'bash ./scripts/install-awscli.sh'
                sh 'curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b ${GOPATH}/bin v1.16.0'
                sh 'curl -L https://github.com/google/protobuf/releases/download/v3.6.1/protoc-3.6.1-linux-x86_64.zip -o /tmp/protoc.zip'
                sh 'unzip /tmp/protoc.zip -d "$HOME"/protoc'

                // TODO: lock these to specific version
                sh 'go get github.com/ckaznocha/protoc-gen-lint'
                sh 'go get github.com/nilslice/protolock/cmd/protolock'
                sh 'go get github.com/mfridman/tparse'

                sh 'go version'
            }
        }

        stage('Build') {
            steps {
                checkout scm
                sh 'go mod download'

                sh 'go install -v -race ./...'
                sh 'make install-sim'
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
                    steps {
                      sh 'go test -vet=off -json -race ./... | go run ./scripts/xunit.go -out tests.xml'
                    }
                }
            }
        }

        stage('Integration') {
            // cannot run in parallel, because tests may end up using ports that
            // test-sim needs.
            steps {
                sh 'make test-sim'
            }
        }
    }

    post {
      always {
        junit 'tests.xml'
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
