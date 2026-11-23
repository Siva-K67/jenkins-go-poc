pipeline {
    // Run the steps inside a temporary Go container
    agent {
        docker { image 'golang:1.23' }
    }

    // Tell Go to keep its cache inside the workspace folder
    environment {
        HOME    = "${WORKSPACE}"
        GOCACHE = "${WORKSPACE}/.cache/go-build"
        GOPATH  = "${WORKSPACE}/.gopath"
    }

    stages {
        stage('Test') {
            steps {
                sh 'go version'
                sh 'go test ./... -v'
            }
        }
    }

    // Runs after the stages, based on the result
    post {
        success { echo 'Tests passed' }
        failure { echo 'Tests failed - check the console output' }
    }
}