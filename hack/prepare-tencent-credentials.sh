#!/bin/bash

echo "secretID: ${TENCENTCLOUD_SECRET_ID}\nsecretKey: ${TENCENTCLOUD_SECRET_KEY}\n" > tencent-credentials.conf
kubectl create namespace vela-system
kubectl create secret generic tencent-account-creds -n vela-system --from-file=credentials=tencent-credentials.conf
rm -f tencent-credentials.conf
