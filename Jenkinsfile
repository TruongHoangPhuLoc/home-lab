pipeline {
    agent any
    stages {
        stage('Test'){
            steps{
                    withKubeConfig([credentialsId: 'prod-cluster-configfile']) {
                        sh 'kubectl cluster-info'
                        sh 'ls -lah'
                }
            }
        }
    }
}
