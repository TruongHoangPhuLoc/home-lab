pipeline {
    agent any
    stages {
        stage('Test'){
                    withKubeConfig([credentialsId: 'prod-cluster-configfile']) 
                    {
                        sh 'kubectl get nodes'
                    }
        }
    }
}
