#!/bin/bash
week_number_in_month=$((($(date +%-d) - 1) / 7 + 1))
backup_base_path=/mnt/backup/$(date +%Y)/$(date +%Y)"-"$(date +%m)/W$week_number_in_month
echo "*********************************************************************************************************************************************************************************************************************************"
echo "Start the incremental backup process"
if [ "$(date +%u)" -eq 7 ]; then
    # If today's Sunday, do incremental backup and prepare, including rollback all uncommitted transactions
    # This prepares for the a new full backup performed nextday
    docker run --name pxb --rm \
    -v /mnt/data/mysql:/mnt/data/mysql/ \
    -v /var/run/mysqld/mysqld.sock:/var/run/mysqld/mysqld.sock \
    -v $backup_base_path:/backup --user root percona/percona-xtrabackup:8.0.35-31 \
    /bin/bash -c "xtrabackup --socket=/var/run/mysqld/mysqld.sock \
    --backup --incremental-basedir=/backup/base \
    --datadir=/mnt/data/mysql \
    --target-dir=/backup/$(date +%u) \
    --user=root --password=root && \
    xtrabackup --prepare \
    --target-dir=/backup/base \
    --incremental-dir=/backup/$(date +%u)"
else
    # If above statement's not true, do incremental backup as usual 
    # but rollingback uncommitted transactions to allow further incremental ones
    # to be able to execute their task
    docker run --name pxb --rm \
    -v /mnt/data/mysql:/mnt/data/mysql/ \
    -v /var/run/mysqld/mysqld.sock:/var/run/mysqld/mysqld.sock \
    -v $backup_base_path:/backup --user root percona/percona-xtrabackup:8.0.35-31 \
    /bin/bash -c "xtrabackup --socket=/var/run/mysqld/mysqld.sock \
    --backup --incremental-basedir=/backup/base \
    --datadir=/mnt/data/mysql \
    --target-dir=/backup/$(date +%u) \
    --user=root --password=root && \
    xtrabackup --prepare --apply-log-only \
    --target-dir=/backup/base \
    --incremental-dir=/backup/$(date +%u) " 
fi

if [ $? -eq 0 ]; then
    echo "The backup successfully completed!"
else
    echo "The backup failed, please check log!"
fi
echo "*********************************************************************************************************************************************************************************************************************************"