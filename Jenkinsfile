pipeline {
    agent {
        docker { image 'golang:1.23' }
    }

    // ADDED: stop a stuck build. Jenkins aborts it after 2 minutes (a normal run takes about 20 seconds).
    options {
        timeout(time: 2, unit: 'MINUTES')
    }

    // ADDED: an input that the caller can send when starting the job
    parameters {
        string(
            name: 'INSTANCE_URL',
            defaultValue: 'http://host.docker.internal:8081',
            description: 'Base URL of the server to test'
        )
        // ADDED (PoC): more inputs, to test how several fields and a secret travel
        string(name: 'ENVIRONMENT', defaultValue: '', description: 'Optional: environment name')
        string(name: 'CLIENT_ID', defaultValue: '', description: 'Optional: client id')
        password(name: 'CLIENT_SECRET', defaultValue: '', description: 'Optional: client secret (PoC: test value only)')
        string(name: 'INPUT_JSON', defaultValue: '', description: 'Optional: the whole input as one JSON string')
        // ADDED (PoC): makes one test sleep, to simulate a stuck build
        booleanParam(name: 'SIMULATE_HANG', defaultValue: false, description: 'PoC: simulate a stuck build')
    }

    environment {
        HOME    = "${WORKSPACE}"
        GOCACHE = "${WORKSPACE}/.cache/go-build"
        GOPATH  = "${WORKSPACE}/.gopath"
        // ADDED: pass the input to the Go tests as an env var
        AIP_INSTANCE_URL = "${params.INSTANCE_URL}"
        // ADDED (PoC): the extra inputs, also as env vars
        AIP_ENVIRONMENT   = "${params.ENVIRONMENT}"
        AIP_CLIENT_ID     = "${params.CLIENT_ID}"
        AIP_CLIENT_SECRET = "${params.CLIENT_SECRET}"
        AIP_INPUT_JSON    = "${params.INPUT_JSON}"
        AIP_SIMULATE_HANG = "${params.SIMULATE_HANG}"
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
                // ADDED: install gotestsum into the workspace.
                // Pinned to v1.12.3 because newer versions need Go 1.24+ and this image is Go 1.23.
                sh 'go install gotest.tools/gotestsum@v1.12.3'
                // ADDED: run tests with gotestsum, write a JSON file and a JUnit XML file
                // -count=1 turns off Go's test cache, so tests really run (and hit the server) every build
                // --format standard-verbose prints the "=== RUN" and "--- PASS" lines live in the console,
                // so the console log can be read while the build runs (PoC Task 6).
                // Do NOT add -v here: gotestsum already runs go test -json, and an extra -v makes Go
                // hold back subtest results until the parent test ends (they would not be live).
                sh '$GOPATH/bin/gotestsum --format standard-verbose --jsonfile results.json --junitfile results.xml -- ./... -count=1'
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