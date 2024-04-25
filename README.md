# SSH access to EC2 instances
export CAPIDEPLOY_SSH_USER=ubuntu
# AWS keypair created for SSH acess
export CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME=sampledeployment005-root-key
# Exported PEM file with private SSH key from the AWS keypair
export CAPIDEPLOY_SSH_PRIVATE_KEY_PATH=~/.ssh/sampledeployment005_rsa

export CAPIDEPLOY_RABBITMQ_ADMIN_NAME=...
export CAPIDEPLOY_RABBITMQ_ADMIN_PASS=...
export CAPIDEPLOY_RABBITMQ_USER_NAME=...
export CAPIDEPLOY_RABBITMQ_USER_PASS=...

# arn:aws:iam::aws_account:user/capillaries-testuser to access s3
# ~/.aws/credentials: default/aws_access_key_id, default/aws_secret_access_key
export CAPIDEPLOY_IAM_AWS_ACCESS_KEY_ID=AK...
export CAPIDEPLOY_IAM_AWS_SECRET_ACCESS_KEY=6vt...
# ~/.aws/config: default/region
export CAPIDEPLOY_DEFAULT_REGION=us-east-1

# Path to local copy of github.com/capillariesio/capillaries
export CAPIDEPLOY_CAPILLARIES_ROOT_DIR=/mnt/c/Users/John Doe/src/capillaries
