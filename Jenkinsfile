def vers
def outFile
def release = false
pipeline {
    agent any
    tools {
        go 'Go 1.17'
    }
    environment {
        NEXUS_CREDS = credentials('1fdafbe4-5486-4347-ac7d-acb9d6710079')
    }
    stages {
        stage("pre") {
            steps {
                script {
                    if (env.TAG_NAME) {
                        vers = "${env.TAG_NAME}"
                        release = true
                    } else {
                        vers = "${env.GIT_COMMIT}"
                    }
                    outFile = "Vili-${vers}"
                    echo "New file: ${outFile}"
                }
            }
        }
        stage("test") {
            steps {
                script {
                    testApp()
                }
            }
        }
        stage("build") {
            steps {
                script {
                    echo "V: ${vers}"
                    echo "File: ${outFile}"
                    buildApp(outFile)
                }
            }
        }
        stage("deploy") {
            steps {
                script {
                    deployApp(outFile, vers, release, NEXUS_CREDS)
                }
            }
        }
    }
}

def testApp() {
    echo 'testing the application...'
    sh './testRecursive.sh'
}

def buildApp(outFile) {
    echo 'building the application...'
    sh 'ls'
    sh "CGO_ENABLED=0 GOOD=linux GOARCH=amd64 go build -o ${outFile}"
    sh 'ls'
}

def deployApp(outFile, vers, release, creds) {
    echo 'deplying the application...'
    echo "deploying version ${vers}"
    if (release) {
        sh "curl -v -u ${creds} --upload-file ${outFile} https://mvnrepo.cantara.no/content/repositories/releases/no/cantara/vili/vili/${vers}/${outFile}"
    } else {
        sh "curl -v -u ${creds} --upload-file ${outFile} https://mvnrepo.cantara.no/content/repositories/snapshots/no/cantara/vili/vili/${vers}/${outFile}"
    }
    sh "rm ${outFile}"
}
