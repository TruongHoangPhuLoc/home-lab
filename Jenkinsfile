pipeline {
    agent any
    stages {
        stage('Test'){
            steps{
                    withKubeConfig([credentialsId: 'prod-cluster-configfile']) {
                        sh 'kubectl apply -f my-kubernetes-directory'
                }
            }
        }
    }
}
