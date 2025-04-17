#!/bin/bash

# --- Config ---
CHART_VERSION=20.12.0
# IMAGE_NAME=docker.io/bitnami/redis
# IMAGE_TAG=7.0.12
# CONTROLLER_NODE=user@controller-node-ip
# TMP_DIR=/tmp/helm_preload

# # --- Prepare ---
# mkdir -p $TMP_DIR
# cd $TMP_DIR

echo "Pulling Helm chart..."
helm pull oci://registry-1.docker.io/bitnamicharts/redis --version ${CHART_VERSION}

# --- Copy to Controller ---
# echo "Copying files to controller node..."
# scp redis-${CHART_VERSION}.tgz redis_image.tar ${CONTROLLER_NODE}:/tmp/

# --- SSH into Controller and load/install ---
# ssh ${CONTROLLER_NODE} bash << EOF
#   echo "Loading Docker image on controller..."
#   docker load -i /tmp/redis_image.tar

#   echo "Installing Helm chart..."
#   helm install ${CHART_NAME} /tmp/redis-${CHART_VERSION}.tgz --set image.repository=${IMAGE_NAME} --set image.tag=${IMAGE_TAG} --set image.pullPolicy=IfNotPresent
# EOF

# --- Cleanup ---
# echo "Done. You can clean up /tmp/helm_preload if desired."