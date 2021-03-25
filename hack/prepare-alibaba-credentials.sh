#!/bin/bash

echo -e "accessKeyId: ${ALICLOUD_ACCESS_KEY}\naccessKeySecret: ${ALICLOUD_SECRET_KEY}" > alibaba-credentials.conf
kubectl create secret generic alibaba-account-creds -n vela-system --from-file=credentials=alibaba-credentials.conf
rm -f alibaba-credentials.conf