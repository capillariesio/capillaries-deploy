# Make it as idempotent as possible, it can be called over and over

# Logrotate
LOGROTATE_CONFIG_FILE=/etc/logrotate.d/cassandra_logrotate.conf

sudo rm -f $LOGROTATE_CONFIG_FILE
sudo tee $LOGROTATE_CONFIG_FILE <<EOF
/var/log/cassandra/* {
    create 0644 root root
    hourly
    rotate 10
    missingok
    notifempty
    compress
    sharedscripts
    postrotate
        echo "Log rotated" > /var/log/cassandra
    endscript
}
EOF

# Logrotate/Cron
# Make sure less /etc/cron.daily/logrotate has something like this (should be installed by logrotate installer):
# #!/bin/sh
# /usr/sbin/logrotate -s /var/lib/logrotate/logrotate.status /etc/logrotate.conf
# EXITVALUE=$?
# if [ $EXITVALUE != 0 ]; then
#     /usr/bin/logger -t logrotate "ALERT exited abnormally with [$EXITVALUE]"
# fi
# exit 0
