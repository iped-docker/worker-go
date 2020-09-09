pipeline {
  agent {
   docker {
     image 'golang:alpine'
   }
  }
  stages {
    stage('Build'){
      environment{
        CGO_ENABLED=0
      }
      steps{
        sh 'go generate'
        sh 'go build'
        archiveArtifacts artifacts: 'worker-go', fingerprint: true
      }
    }
  }
}