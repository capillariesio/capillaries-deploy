if [ "$SSH_USER" = "" ]; then
  echo Error, missing: SSH_USER=ubuntu
 exit 1
fi

processid=$(pgrep capiwebapi)
if [ "$processid" = "" ]; then
  /home/$SSH_USER/bin/capiwebapi >> /mnt/capi_log/capiwebapi.log 2>&1 &
fi