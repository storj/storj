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
                        sh 'go run scripts/use-ports.go -from 1024 -to 10000 &'
                        sh 'go test -vet=off -json -race ./... | go run ./scripts/xunit.go -out tests.xml'
                    }

                    post {
                        always {
                            junit 'tests.xml'
                        }
                    }
                }

                stage('Integration') {
                    environment {
                        // use different hostname to avoid port conflicts
                        STORJ_NETWORK_HOST4 = '127.0.0.2'
                        STORJ_NETWORK_HOST6 = '127.0.0.2'
                    }

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