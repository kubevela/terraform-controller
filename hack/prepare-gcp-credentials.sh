#!/bin/bash

echo "gcpCredentialsJSON: '${GOOGLE_CREDENTIALS}'\ngcpProject: ${GOOGLE_PROJECT}" > gcp-credentials.conf
kubectl create secret generic gcp-account-creds -n vela-system --from-file=credentials=gcp-credentials.conf
rm -f gcp-credentials.conf