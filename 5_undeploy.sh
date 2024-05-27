##!/bin/bash

set +e # Continue on failure
set -x # Print commands

./capideploy stop_services "*" -prj=sample.json --verbose > undeploy.log

set -e # Exit on failure
./capideploy detach_volumes "bastion" -prj=sample.json --verbose >> undeploy.log
./capideploy delete_volumes "*" -prj=sample.json --verbose >> undeploy.log
./capideploy delete_instances "*" -prj=sample.json --verbose >> undeploy.log

./capideploy delete_security_groups -prj=sample.json --verbose >> undeploy.log
./capideploy delete_networking -prj=sample.json --verbose >> undeploy.log
./capideploy delete_floating_ips -prj=sample.json --verbose >> undeploy.log

./capideploy list_deployment_resources -prj=./sample.json
