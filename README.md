# IAM settings

You can run capideploy under your AWS root account, but this is generally discouraged. Let's pretend that capideploy is executed by a third party and you want to grant that third party some specific permissions that allow that third party to create Capillaries deployment in AWS. Giving a third party access to your AWS resources is a standard practice and the recommended way to do that is to use IAM roles. This  section discusses the AWS IAM preparation steps to create the necessary role structure. Basic familiarity with AWS console is required.

## Users and groups

Let's assume all capideploy activities are performed on behalf of an IAM user named `UserCapideployOperator`. As a first step, create this user in `IAM->Users` section of AWS console. In `IAM->User groups`, create a group `GroupCapideployOperators` and add `UserCapideployOperator` to it.

## Policies and roles

### PolicyAccessCapillariesTestbucket and RoleAccessCapillariesTestbucket

Your AWS deployment will need to read and write files from/to S3 bucket. As per [Capillaries S3 instructions](https://github.com/capillariesio/capillaries/blob/main/doc/s3.md), we assume that you already have an S3 bucket for your future Capillaries deployment, let's assume the name of the bucket is `capillaries-testbucket` and it has `Block all public access` setting on. And here is the key difference:
- Capillaries test S3 bucket access described in that doc uses user-based access model (bucket policy explicitly gives the user `arn:aws:iam::<your_aws_acount>:user/UserAccessCapillariesTestbucket` access to the bucket);
- capideploy S3 bucket access model uses a separate policy and a separate role with this policy attached, and Capillaries instances can assume that role.

In `IAM->Policies`, let's create a policy `PolicyAccessCapillariesTestbucket` that allows access to the bucket we will be using:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "s3:ListBucket",
            "Resource": "arn:aws:s3:::capillaries-testbucket"
        },
        {
            "Effect": "Allow",
            "Action": [
                "s3:DeleteObject",
                "s3:GetObject",
                "s3:PutObject"
            ],
            "Resource": "arn:aws:s3:::capillaries-testbucket/*"
        }
    ]
}
```

In `IAM->Roles`, create a role `RoleAccessCapillariesTestbucket` with `Trusted entity type` set to `AWS Service` and:
- attach the newly created `PolicyAccessCapillariesTestbucket` to it (`Permissions` tab);
- under `Trust relationships`, make sure that ec2 service can assume this role:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": "ec2.amazonaws.com"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}
```

Please note that, since we created therole with `Trusted entity type` set to `AWS Service`, `RoleAccessCapillariesTestbucket` has two ARNa, as a role and as an instance profile:

| Name type | Name |
| - | - |
| ARN | arn:aws:iam::<your_aws_account_id>:role/RoleAccessCapillariesTestbucket |
| Instance profile ARN | arn:aws:iam::<your_aws_account_id>:instance-profile/RoleAccessCapillariesTestbucket |

Run the following command as AWS root or as `UserCapideployOperator` (if you have already assigned `iam:GetInstanceProfile` permission to it, see below):

```
$ aws iam get-instance-profile --instance-profile-name RoleAccessCapillariesTestbucket
```

The result shows that role `RoleAccessCapillariesTestbucket` is "wrapped" by instance profile `RoleAccessCapillariesTestbucket`.

### PolicyCapideployOperator

As we agreed above, `UserCapideployOperator` (who potentially can be a third party), needs only a very restricted set of permissions. This user will need permissions to do two major things:
- create/delete AWS resources (networks, subnets, instances etc) that will provide infrastructure to run Capillaries binaries and Cassandra cluster
- give created instances permission to read/write config/data files from/to S3 bucket

In IAM->Policies, create a customer-managed policy PolicyCapideployOperator:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "PolicyCapideployOperatorCreateInfra",
            "Effect": "Allow",
            "Action": [
                "ec2:AllocateAddress",
                "ec2:AssociateAddress",
                "ec2:AssociateIamInstanceProfile",
                "ec2:AssociateRouteTable",
                "ec2:AttachInternetGateway",
                "ec2:AttachVolume",
                "ec2:AuthorizeSecurityGroupIngress",
                "ec2:CreateImage",
                "ec2:CreateInternetGateway",
                "ec2:CreateNatGateway",
                "ec2:CreateRoute",
                "ec2:CreateRouteTable",
                "ec2:CreateSecurityGroup",
                "ec2:CreateSubnet",
                "ec2:CreateTags",
                "ec2:CreateVolume",
                "ec2:CreateVpc",
                "ec2:DeleteInternetGateway",
                "ec2:DeleteNatGateway",
                "ec2:DeleteRouteTable",
                "ec2:DeleteSecurityGroup",
                "ec2:DeleteSnapshot",
                "ec2:DeleteSubnet",
                "ec2:DeleteVolume",
                "ec2:DeleteVpc",
                "ec2:DeregisterImage",
                "ec2:DescribeAddresses",
                "ec2:DescribeImages",
                "ec2:DescribeInstances",
                "ec2:DescribeInstanceTypes",
                "ec2:DescribeInternetGateways",
                "ec2:DescribeKeyPairs",
                "ec2:DescribeNatGateways",
                "ec2:DescribeRouteTables",
                "ec2:DescribeSecurityGroups",
                "ec2:DescribeSnapshots",
                "ec2:DescribeSubnets",
                "ec2:DescribeTags",
                "ec2:DescribeVolumes",
                "ec2:DescribeVpcs",
                "ec2:DetachInternetGateway",
                "ec2:DetachVolume",
                "ec2:ReleaseAddress",
                "ec2:RunInstances",
                "ec2:TerminateInstances",
                "iam:GetInstanceProfile",
                "tag:GetResources"
            ],
            "Resource": "*"
        },
        {
            "Sid": "PolicyCapideployOperatorPassRoleAccessBucket",
            "Effect": "Allow",
            "Action": "iam:PassRole",
            "Resource": "arn:aws:iam::<your_aws_account_id>:role/RoleAccessCapillariesTestbucket"
        }
    ]
}
```

The first part is obvious: it lists all AWS API calls performed by capideploy. As for the second part,it adds PassRole permission for `RoleAccessCapillariesTestbucket` created above. Without this permission, `AssociateIamInstanceProfile` call (that tells AWS to allow instances to access the bucket) will fail.

Just in case - to list all AWS API calls used by capideploy, run:
```shell
grep -r -e "ec2Client\.[A-Za-z]*" --include "*.go"
grep -r -e "tClient\.[A-Za-z]*" --include "*.go"
```

## Attach PolicyCapideployOperator to GroupCapideployOperators

In `IAM->User groups->GroupCapideployOperators->Permissions`, attach `PolicyCapideployOperator`.

# Environment variables used by Capideploy

Sample .rc file to run before Capildeploy contains variables used in the .jsonnet file:
```
# SSH access to EC2 instances
export CAPIDEPLOY_SSH_USER=ubuntu
# Name of the keypair stored at AWS
export CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME=sampledeployment005-root-key
# Exported PEM file with private SSH key from the AWS keypair
export CAPIDEPLOY_SSH_PRIVATE_KEY_PATH=/home/johndoe/.ssh/sampledeployment005_rsa

# NGINX IP address filter: your IP address(es) or cidr(s), for example: "135.23.0.0/16,136.104.0.21"
export CAPIDEPLOY_BASTION_ALLOWED_IPS="..."
export CAPIDEPLOY_EXTERNAL_WEBAPI_PORT=6544

# This is where capideploy takes Capillaries binaries from,
# see https://github.com/capillariesio/capillaries/blob/main/binaries_upload.sh
export CAPIDEPLOY_CAPILLARIES_RELEASE_URL=https://capillaries-release.s3.us-east-1.amazonaws.com/latest

# RabbitMQ admin access (RabbitMQ Mgmt UI)
export CAPIDEPLOY_RABBITMQ_ADMIN_NAME=...
export CAPIDEPLOY_RABBITMQ_ADMIN_PASS=...

# RabbitMQ user access (used by Capillaries components to talk to RabbitMQ)
export CAPIDEPLOY_RABBITMQ_USER_NAME=...
export CAPIDEPLOY_RABBITMQ_USER_PASS=...

# ~/.aws/config: default/region (without it, AWS API will not locate S3 buckets)
export CAPIDEPLOY_S3_AWS_DEFAULT_REGION=us-east-1

# Capideploy will use this instance profile when creating instances that need access to S3 bucket
export CAPIDEPLOY_INSTANCE_PROFILE_WITH_S3_ACCESS=RoleAccessCapillariesTestbucket
```

# Create deployment

Run `1_deploy.sh`. If everything goes well, it will create a Capillaries deployment accessible at BASTION_IP address returned by `1_deploy.sh` (capideploy does not use DNS, so you will have to access your deployment by IP address).

# Processing data using created deployment

[Capillaries repository](https://github.com/capillariesio/capillaries) has a few tests that are ready to run in the cloud deployment:
- [lookup quicktest S3](https://github.com/capillariesio/capillaries/tree/main/test/code/lookup/quicktest_s3): run `1_create_data_s3.sh` and `2_one_run_cloud.sh`
- [Fannie Mae quicktest S3](https://github.com/capillariesio/capillaries/tree/main/test/code/fannie_mae/quicktest_s3): run `1_copy_data_s3.sh` and `2_one_run_cloud.sh`
- [Fannie Mae bigtest](https://github.com/capillariesio/capillaries/tree/main/test/code/fannie_mae/bigtest): run `1_copy_data.sh` and `2_one_run_cloud.sh`
- [Portfolio bigtest](https://github.com/capillariesio/capillaries/tree/main/test/code/portfolio/bigtest): run `1_create_data.sh` and `2_one_run_cloud.sh`

You will probably have to run these tests using `UserAccessCapillariesTestbucket` IAM user as per [Capillaries S3 instructions](https://github.com/capillariesio/capillaries/blob/main/doc/s3.md): that user should have access to the S3 bucket to upload/download config/data files. 

Please note that in order to run these tests or your own scripts in your newly created deployment you only need access to the S3 bucket and HTTP access to the bastion host (which should allow HTTP access from all machines matching CAPIDEPLOY_BASTION_ALLOWED_IPS address or cidr). `UserCapideployOperator` user is not involved at this point.

In general, you can start a Capillaries run in your deployment via REST API as follows:

```shell
CAPILLARIES_AWS_TESTBUCKET=capillaries-testbucket
keyspace="lookup_quicktest_s3"
cfgS3=s3://$CAPILLARIES_AWS_TESTBUCKET/capi_cfg/lookup_quicktest
outS3=s3://$CAPILLARIES_AWS_TESTBUCKET/capi_out/lookup_quicktest
scriptFile=$cfgS3/script.json
paramsFile=$cfgS3/script_params_one_run_s3.json
webapiUrl=http://$BASTION_IP:6544
startNodes=read_orders,read_order_items
curl -s -w "\n" -d '{"script_uri":"'$scriptFile'", "script_params_uri":"'$paramsFile'", "start_nodes":"'$startNodes'"}' -H "Content-Type: application/json" -X POST $webapiUrl"/ks/$keyspace/run"
```

# Delete deployment

To delete all AWS resources that your deployment uses, run `5-undeploy.sh`.