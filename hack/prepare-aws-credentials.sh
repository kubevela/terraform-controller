#!/bin/bash

# refer to https://registry.terraform.io/providers/hashicorp/aws/latest/docs
echo "awsAccessKeyID: ${AWS_ACCESS_KEY_ID}\nawsSecretAccessKey: ${AWS_SECRET_ACCESS_KEY}" > aws-credentials.conf
kubectl create secret generic aws-account-creds -n vela-system --from-file=credentials=aws-credentials.conf
rm -f aws-credentials.conf