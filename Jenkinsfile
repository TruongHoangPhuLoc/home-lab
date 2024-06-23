pipeline {
    agent any
    stages {
        stage('Test'){
            steps{
                    withKubeConfig([credentialsId: 'prod-cluster-configfile-1']) {
                        sh 'kubectl get nodes'
                }
            }
        }
    }
}
