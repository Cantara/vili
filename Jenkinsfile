def gitV
pipeline {
    agent any
    tools {
        go 'Go 1.17'
    }
    stages {
        stage("pre") {
            steps {
                script {
                    echo "V: ${gitV} ${VERSION} ${TAG_NAME}"
                    gitV = sh(script: "git describe --abbrev=0 --tags", returnStdout: true).toString().trim()
                }
            }
        }
        stage("build") {
            steps {
                script {
                    echo "V: ${gitV} ${VERSION}"
                    buildApp()
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
                    deployApp()
                }
            }
        }
    }
}
def buildApp() {
    echo 'building the application...'
    sh 'ls'
    echo "cd 'src'"
    sh 'ls'
    sh 'cd src && go build'
    sh 'ls'
}

def testApp() {
    echo 'testing the application...'
    echo 'function recursive_for_loop {ls -1| while read f; do; if [ -d $f  -a ! -h $f ]; then; cd -- "$f"; echo "Doing something in folder `pwd`/$f"; recursive_for_loop; cd ..; fi;  done; }; recursive_for_loop'
}

def deployApp() {
    echo 'deplying the application...'
    echo "deploying version ${params.VERSION}"
}
