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
        AIP_INSTANCE_URL = "${params.INSTANCE_URL}"
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
                sh 'echo "Testing against: $AIP_INSTANCE_URL"'
            }
        }
        stage('Test') {
            steps {
                sh 'go version'
                // ADDED: install gotestsum (pinned version) into the workspace
                sh 'go install gotest.tools/gotestsum@v1.13.0'
                // ADDED: run tests with gotestsum, write a JSON file and a JUnit XML file
                sh '$GOPATH/bin/gotestsum --jsonfile results.json --junitfile results.xml -- ./... -v'
            }
        }
    }

    post {
        // ADDED: always publish reports, even when tests fail
        always {
            junit allowEmptyResults: true, testResults: 'results.xml'
            archiveArtifacts artifacts: 'results.json,results.xml', allowEmptyArchive: true
        }
        success { echo 'Tests passed' }
        failure { echo 'Tests failed - check the console output' }
    }
}