from common import *
from envs import *

REDIS_IMAGE = "bitnami/redis:7.4.2-debian-12-r6"
REDIS_TAR = "redis.tar"
TMP_PATH = f"/tmp/{REDIS_TAR}"

def preload_redis_image():
    # Step 1: Pull and save locally
    print(f"Pulling Redis image: {REDIS_IMAGE}")
    subprocess.run(f"docker pull {REDIS_IMAGE}", shell=True, check=True)

    print("Saving Redis image to tarball...")
    subprocess.run(f"docker save {REDIS_IMAGE} -o {REDIS_TAR}", shell=True, check=True)

def preload_to_node(node):
    remote_path = f"{CLOUDLAB_USER}@{node}:{TMP_PATH}"

    print(f"🚀 Copying Redis image to {node}")
    subprocess.run(
        f'scp -o StrictHostKeyChecking=no -i {KEYFILE} {REDIS_TAR} {remote_path}',
        shell=True,
        check=True
    )

    print(f"📦 Importing image on {node} using containerd...")
    subprocess.run(
        f'ssh -o StrictHostKeyChecking=no -i {KEYFILE} {CLOUDLAB_USER}@{node} '
        f'"ctr -n k8s.io images import {TMP_PATH} && rm {TMP_PATH}"',
        shell=True,
        check=True
    )

def main():   
    # Preload Redis image
    # preload_redis_image()

    for node in SERVERS:
        preload_to_node(node)

if __name__ == '__main__':
    main()