#!/bin/bash

echo "vSphereUser: ${VSPHERE_USER}\nvSpherePassword: ${VSPHERE_PASSWORD}\nvSphereServer: ${VSPHERE_SERVER}\nvSphereAllowUnverifiedSSL: ${VSPHERE_ALLOW_UNVERIFIED_SSL}" > vsphere-credentials.conf
kubectl create secret generic vsphere-account-creds -n vela-system --from-file=credentials=vsphere-credentials.conf
rm -f vsphere-credentials.con