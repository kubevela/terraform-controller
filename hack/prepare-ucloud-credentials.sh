#!/bin/bash

echo "publicKey: ${UCLOUD_PUBLIC_KEY}\nprivateKey: ${UCLOUD_PRIVATE_KEY}\nregion: ${UCLOUD_REGION}\nprojectID: ${UCLOUD_PROJECT_ID}" > ucloud-credentials.conf
kubectl create namespace vela-system
kubectl create secret generic ucloud-account-creds -n vela-system --from-file=credentials=ucloud-credentials.conf
rm -f ucloud-credentials.conf
