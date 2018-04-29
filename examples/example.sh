#!/bin/bash

export NGINX_IMAGE_TAG="1.11-alpine"
#kd -c mykube -namespace testing -f nginx-deployment.yaml --debug-templates --dryrun

export LIST="ENTRY1,ENTRY2,ENTRY3,ENTRY4"
#kd -c mykube -n testing -f split.yaml --debug-templates --dryrun

export BAR="${PWD}/vars/config"
#kd -c mykube -n testing -f file.yaml --debug-templates --dryrun

kd -f . --debug-templates --dryrun

echo "Dry run example complete"
