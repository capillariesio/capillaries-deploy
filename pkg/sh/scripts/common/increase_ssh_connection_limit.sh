# Default ssh connection limit from one client is 10, increase it
sudo sed -i -e "s~[# ]*MaxStartups[ ]*[0-9:]*~MaxStartups 1000~g" /etc/ssh/sshd_config
sudo sed -i -e "s~[# ]*MaxSessions[ ]*[0-9]*~MaxSessions 1000~g" /etc/ssh/sshd_config

# Since kinetic, Ubuntu doesn't honour /etc/ssh/sshd_config
# (https://discourse.ubuntu.com/t/sshd-now-uses-socket-based-activation-ubuntu-22-10-and-later/30189/8)
# Since I can't find how to change MaxStartups for ssh.socket, let's roll back to ssh.service: 

# Ignore stderr, this cmd has a habit of throwing " Removed ..."
sudo systemctl disable --now ssh.socket 2>/dev/null
# Ignore stderr, this cmd has a habit of throwing "Synchronizing state of ssh.service with SysV service script with..."
sudo systemctl enable --now ssh.service 2>/dev/null

# Now it's ok to reload (with ssh.socket we get "Unit sshd.service could not be found.")
sudo systemctl reload sshd