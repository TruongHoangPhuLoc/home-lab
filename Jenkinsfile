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
        stage('Reload-Configuration')
        when{
            anyOf{
                changeset "**/monitoring-server/configuration/prometheus/alert_rules/**"
                changeset "**/monitoring-server/configuration/prometheus/recording_rules/**"
            }
        }
        steps{
            sh 'echo Rules Changed'
        }
        stage('Re-Deploy-Compose'){
            when {
                    changeset "**/monitoring-server/configuration/docker-compose.yml"
            }
            steps{
                    sh 'echo Compose file changed'
            }
        }
    }
}

