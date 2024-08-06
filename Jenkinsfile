pipeline {
    agent any
    // stages {
    //     stage('Test'){
    //         steps{
    //                 withKubeConfig([credentialsId: 'prod-cluster-configfile']) {
    //                     sh 'kubectl cluster-info'
    //                     sh 'echo $KUBECONFIG'
    //                     sh 'cat $KUBECONFIG'
    //             }
    //         }
    //     }
    // }

    stages {
        stage('Test-Changes'){
            when 
            {  allOf {
                    changeset "**/monitoring-server/configuration/**"
                    not {
                        changeset "**/monitoring-server/configuration/prometheus/targets/**"
                    }
                }
            }
            steps{
                        sh 'echo Changes applied to Monitoring'
            }
        }
    }
}

