#!bin/bash
week_number_in_month=$((($(date +%-d) - 1) / 7 + 1))
backup_base_path=/mnt/backup/$(date +%Y)/$(date +%Y)"-"$(date +%m)/W$week_number_in_month
#Create a full backup on created folder
echo "*********************************************************************************************************************************************************************************************************************************"
echo "Start the full backup process..." | logger
docker run --name pxb --rm \
 -v /var/run/mysqld/mysqld.sock:/var/run/mysqld/mysqld.sock \
 -v /mnt/data/mysql:/mnt/data/mysql/ \
 -v $backup_base_path:/backup --user root percona/percona-xtrabackup:8.0.35-31 \
  /bin/bash -c "xtrabackup --socket=/var/run/mysqld/mysqld.sock \
  --backup --datadir=/mnt/data/mysql/ \
  --target-dir=/backup/base \
  --user=root --password=root && \
  xtrabackup --prepare --apply-log-only \
  --target-dir=/backup/base"
  
if [ $? -eq 0 ]; then
    echo "The backup successfully completed!"
else
    echo "The backup failed, please check log!"
fi
echo "*********************************************************************************************************************************************************************************************************************************"
