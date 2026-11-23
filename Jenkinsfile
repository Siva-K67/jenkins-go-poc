pipeline {
    agent {
        docker { image 'golang:1.23' }
    }

    // ADDED: an input that the caller can send when starting the job
    parameters {
        string(
            name: 'INSTANCE_URL',
            defaultValue: 'http://host.docker.internal:8081',
            description: 'Base URL of the server to test'
        )
    }

    environment {
        HOME    = "${WORKSPACE}"
        GOCACHE = "${WORKSPACE}/.cache/go-build"
        GOPATH  = "${WORKSPACE}/.gopath"
        // ADDED: pass the input to the Go tests as an env var
        INSTANCE_URL = "${params.INSTANCE_URL}"
    }

    stages {
        // ADDED: stop early if the URL is empty, and print it
        stage('Check input') {
            steps {
                script {
                    if (!params.INSTANCE_URL?.trim()) {
                        error('INSTANCE_URL is empty')
                    }
                }
                sh 'echo "Testing against: $INSTANCE_URL"'
            }
        }
        stage('Test') {
            steps {
                sh 'go version'
                sh 'go test ./... -v'
            }
        }
    }

    post {
        success { echo 'Tests passed' }
        failure { echo 'Tests failed - check the console output' }
    }
}