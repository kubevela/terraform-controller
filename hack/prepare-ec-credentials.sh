#!/bin/bash

echo "ecApiKey: '${EC_API_KEY}'" > ec-credentials.conf
kubectl create secret generic ec-account-creds -n vela-system --from-file=credentials=ec-credentials.conf
rm -f ec-credentials.conf
