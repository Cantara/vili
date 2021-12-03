def vers
def outFile
pipeline {
    agent any
    tools {
        go 'Go 1.17'
    }
    stages {
        stage("pre") {
            steps {
                script {
                    if (env.TAG_NAME) {
                        vers = "${env.TAG_NAME}"
                    } else {
                        vers = "${env.GIT_COMMIT}"
                    }
                    outFile = "Vili-${vers}"
                    echo "New file: ${outFile}"
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
        stage("test") {
            steps {
                script {
                    testApp()
                }
            }
        }
        stage("deploy") {
            steps {
                script {
                    deployApp(outFile, vers)
                }
            }
        }
    }
}
def buildApp(outFile) {
    echo 'building the application...'
    sh 'ls'
    sh "CGO_ENABLED=0 GOOD=linux GOARCH=amd64 go build -o ${outFile}"
    sh 'ls'
}

def testApp() {
    echo 'testing the application...'
    echo 'function recursive_for_loop {ls -1| while read f; do; if [ -d $f  -a ! -h $f ]; then; cd -- "$f"; echo "Doing something in folder `pwd`/$f"; recursive_for_loop; cd ..; fi;  done; }; recursive_for_loop'
}

def deployApp(outFile, vers) {
    echo 'deplying the application...'
    echo "deploying version ${vers}"
}
