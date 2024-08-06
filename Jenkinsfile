pipeline {
    agent any
    environment {
        REMOTE_USER = 'locthp'
        REMOTE_SERVER = '172.16.1.215'
        SSH_CREDENTIALS_ID = 'monitoring-host-ssh'
        GIT_REPO_URL = 'https://github.com/TruongHoangPhuLoc/home-lab.git'
        TARGET_DIR = '$(pwd)/home-lab/infrastructure/monitoring-server/configuration'
    }
    stages {
        stage('SSH-To-Monitoring-Host') {
            steps {
                sshagent([SSH_CREDENTIALS_ID]) {
                    script {
                        // Execute commands directly on the Docker host
                        sh '''
                        ssh ${REMOTE_USER}@${REMOTE_SERVER} "whoami"
                        '''
                    }
                }
            }
        }
        stage('Reload-Configuration') {
            when {
                anyOf {
                    changeset "**/monitoring-server/configuration/prometheus/alert_rules/**"
                    changeset "**/monitoring-server/configuration/prometheus/recording_rules/**"
                }
            }
            steps {
                sh 'echo Rules Changed'
            }
        }
        stage('ReDeploy-Monitoring-Stack') {
            when {
                changeset "**/monitoring-server/configuration/docker-compose.yml"
            }
            steps {
                sh 'echo Compose file changed'
            }
        }
    }
}
