pipeline {
  agent any
  stages {
    stage('Test') {
      environment {
        COVERALLS_TOKEN = credentials('COVERALLS_TOKEN')
      }
      steps {
        sh 'make test-docker'
        sh 'make test-captplanet-docker'
      }
    }
    stage('Build Images') {
      steps {
        sh 'make images'
      }
    }

    stage('Build Binaries') {
      steps {
		sh 'make binaries'
      }
    }

    stage('Push Images') {
      when {
        branch 'master'
      }
      steps {
        echo 'Push to Repo'
        sh 'make push-images'
      }
    }

    /* This should only deploy to staging if the branch is master */
    stage('Deploy') {
      when {
        branch 'master'
      }
      steps {
        sh 'make deploy'
      }
    }

  }
  post {
    failure {
      echo "Caught errors! ${err}"
    }
    cleanup {
      sh 'make test-docker-clean clean-images'
      deleteDir()
    }
  }
}
