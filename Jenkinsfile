pipeline {
    agent any
    environment {
        REMOTE_USER = 'locthp'
        REMOTE_SERVER = '172.16.1.215'
        SSH_CREDENTIALS_ID = 'monitoring-host-ssh'
        GIT_REPO_URL = 'https://github.com/TruongHoangPhuLoc/home-lab.git'
        TARGET_DIR = '$(pwd)/home-lab'
    }
    stages {
        stage('SSH-To-Monitoring-Host-And-CheckOut') {
            steps {
                sshagent([SSH_CREDENTIALS_ID]) {
                    script {
                        // Execute commands directly on the Docker host
                        sh '''
                        whoami
                        set +e 
                        ssh -o StrictHostKeyChecking=yes ${REMOTE_USER}@${REMOTE_SERVER} "exit"
                        if [ $? -ne 0 ]; then
                            mkdir -p ~/.ssh
                            ssh-keyscan -H ${REMOTE_SERVER} >> ~/.ssh/known_hosts
                        fi
                        set -e
                        ssh ${REMOTE_USER}@${REMOTE_SERVER} "if [ -d ${TARGET_DIR}  ]; then cd ${TARGET_DIR} && git pull > /dev/null 2>&1; else git clone ${GIT_REPO_URL} > /dev/null 2>&1; fi"
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
                sshagent([SSH_CREDENTIALS_ID]) {
                    script {
                        sh '''
                        ssh ${REMOTE_USER}@${REMOTE_SERVER} "
                        curl -X POST http://localhost:9090/-/reload > /dev/null 2>&1
                        "
                        '''
                    }
                }
            }
        }
        stage('ReDeploy-Monitoring-Stack') {
            when {
                changeset "**/monitoring-server/configuration/docker-compose.yml"
            }
            steps {
                sshagent([SSH_CREDENTIALS_ID]) {
                    script{
                        sh '''
                        ssh ${REMOTE_USER}@${REMOTE_SERVER} "
                        source .secret-env-exporting.sh
                        cd ${TARGET_DIR}/infrastructure/monitoring-server/configuration/ && docker compose down  > /dev/null 2>&1 && docker compose up -d  > /dev/null 2>&1
                        " 
                        '''
                    }
                }
            }
        }
    }
}
