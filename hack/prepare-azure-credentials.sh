#!/bin/bash

echo "armClientID: ${ARM_CLIENT_ID}\narmClientSecret: ${ARM_CLIENT_SECRET}\narmSubscriptionID: ${ARM_SUBSCRIPTION_ID}\narmTenantID: ${ARM_TENANT_ID}" > azure-credentials.conf
kubectl create secret generic azure-account-creds -n vela-system --from-file=credentials=azure-credentials.conf
rm -f azure-credentials.conf
