pipeline {
  agent {
   docker {
     image 'golang:alpine'
   }
  }
  stages {
    stage('Build'){
      environment{
        HOME=/tmp/
        CGO_ENABLED=0
      }
      steps{
        sh 'go build'
        archiveArtifacts artifacts: 'worker-go', fingerprint: true
      }
    }
  }
}
