# Mucache Experiments

## Prerequisites

### Cloudlab Cluster
Initialize a cluster of 13 machines on Cloudlab using the following profile:
https://www.cloudlab.us/p/8e054430c6b73652669ec36f24c1ecb716f80c46

Wait until all nodes are ready, download the minifest file as `./manifest.xml`.

### Controller Machine
The controller send files and commands to all worker machines.
Any machine (e.g. Your host) that has Python3 can acts as the controller.

On the controller

```bash
pip3 install fabric
git clone https://github.com/DKW2/MuCache_Extended
```

copy the minifest file to the MuCache_Extended/

## Setup
On the controller

```bash
cd mucache
export node_username=${username_of_cloudlab}
export private_key=${ssh_key_location}
python3 scripts/host/upload.py
python3 scripts/host/setup.py
```

After it finishes, login into node-0 and you should see the following:

```bash
kubectl get nodes
```

```bash
NAME     STATUS   ROLES           AGE    VERSION
node-0   Ready    control-plane   168m   v1.26.1
node-1   Ready    <none>          168m   v1.26.1
node-2   Ready    <none>          167m   v1.26.1
node-3   Ready    <none>          167m   v1.26.1
node-4   Ready    <none>          167m   v1.26.1
node-5   Ready    <none>          167m   v1.26.1
...
```
Lastly, we want to preload all the helm charts and images as well to avoid pulling them constantly, so run the following within the controller node (node0)
```bash
cd MuCache_Extended
helm pull oci://registry-1.docker.io/bitnamicharts/redis --version 20.12.0
```
We want the helm chart zip within the MuCache folder.

## Run
### Build applications
In this artifact, we have four open-source microservice applications, including SocialMedia,
MovieReview, HotelRes, and OnlineBoutique; we have four synthetic applications, including
Proxy, Chain, Fanout, and Fanin.
To run these applications, you can use our [pre-built images on dockerhub](https://hub.docker.com/repository/docker/tauta/mucache/general).

Alternatively, you can build the images on any x86 machines and push to dockerhub by running
```bash
./scripts/host/build_and_push=${docker_io_username} # application image
./scripts/cm/build_and_push=${docker_io_username} # cache manager image
```

If you're using MuCache's pre-built images, set on the controller node (for example, node0)
```bash
export docker_io_username=tauta
```
If you want to utilize my personal pre-built images, set on the controller node (for example, node0)
```bash
export docker_io_username=fortemir
```

Note: To modify and configure the benchmarks and cache code, you must build the images on DockerHub and use the correct docker_io_username to load the correct images.

### Cache Size (Figure 13)
```bash
python3 experiments/cachesize/run.py
```
The results are printed to the console and saved in `hotel-md.json` and `hotel-md-upper.json`. `hotel-md.json` contains a json map with keys being the cache size in MB and values being the p50 and p95 latency. `hotel-md-upper` is the TTL-inf baseline.

### Microbenchmark (Figure 14)
To run the three microservices, Chain, Fanout and Fanin.
```bash
python3 experiments/chain/run.py # chain
python3 experiments/star/run.py  # fanout
python3 experiments/fanin/run.py # fanin
```

The results will be saved to `{APP}-baseline.json` for baseline and `{APP}.json` for mucache.

### Hitrate (Figure 17)
```bash
python3 experiments/twoservices/hitrate.py
```
The results are saved at `hitrate_hdr_baseline` for the baseline, `hitrate_hdr_{hit_rate}` for mucache and `hitrate_hdr_{hit_rate}_upper` for TTL-inf.

### Real-world applications (Figure 10)
To run the four real-world applications, run
```bash
python3 experiments/movie/run.py
python3 experiments/hotel/run.py
python3 experiments/boutique/run.py
python3 experiments/social/run.py
```
The scripts will print the output and write the results to `{APP}-upper.json`, `{APP}-baseline.json` and `{APP}.json`, which correspond to TTL-inf, baseline and mucache respectively.

### Different TTL baselines (Figure 11)

```bash
python3 experiments/hotel/ttl.py
```
The script will print the output and write and result to `hotel-upper-ttl.json`.

### Sharding (Figure 12)
To build the applications and cache manager for the sharding example, run

```bash
./scripts/host/shard_build_and_push=${docker_io_username} # application image
./scripts/cm/shard_build_and_push=${docker_io_username} # cache manager image
```

## Additional Information
Below are a few things to note when running these experiments smoothly or modifying the code.

### Experiment Workflow
When running an experiment script, it runs the experiment at varying requests per seconds (RPS). It will then output a json file in the main directory containing the latency distribution, throughput and cache hit-rate (if cache is used). These outputs are then used to graph the results seen in the MuCache paper and my paper.

Further delving into the test script, each experiment iteration does the following in run_once():
1. Call the clean() function, which uninstalls every pod (cache, cache manager, service, and redis) before reinstalling the cache and redis pods.
2. Deploy services using scripts/deploy.sh.
3. If data is needed, a populate script is called to populate the microservice with data
4. A proxy server is started, which sends requests to the microservice
5. We use oha (a CLI tool that measures latency) on the proxy server at the given RPS and get the results
6. We parse the results and add in hit rate (if possible)
7. Return results
As we can see, the general experiment workflow has us send requests to the proxy server to then send requests to the microservice. And we measure the total latency from the proxy server.

Most experiments (real-world benchmarks and fanin) utilize a proxy server while Chain and Star utilize a simulated cache hit/miss function. In addition, within my code, the experiment scripts have support for loading different workloads for the proxy server to send, prefetching, and batch invalidation.

### Configurations to Pay Attention to
One issue to deal with when running the experiments is to pay attention to resource consumption. There are two areas where resource consumption can become a bottleneck and mess with your results:
- Configurations of the Docker images (**app.yaml**, cm.yaml, cache.yaml, and redis.yaml)
- Configuration of the Proxy server (# of workers)

To avoid having pods be starved of resources, you have to designate some resource consumption for each pod in their configuration file. Usually, the consumption of resources is ordered with the following: app >>> cm > cache > redis. As we can see, the app service pods require the most computation. Thus, to guarantee they always have resources, we add within the configuration file their CPU and resource allocation. Personally, in my experiments, I found setting the base CPU allocation to 5000mi with max 7000mi works well to prevent the services from bottlenecking. For cm, cache, redis, CPU allocations of 500-1000mi will do just fine. As for memory, it isn't a bottleneck so you could designate it to a sufficienctly high amount.

Next, besides the pods themselves, we also need to account for the resource allocation of our controller node. Because we're sending thousands of requests per second using oha, we can quickly throttle our experiments if we don't have enough workers for the proxy server. Thus, within proxy/src/main.rs, we can set the max_workers parameter to a higher amount (10-12) to fully utilize our resources and prevent throttling.

These were the main two causes of poor results for me. So keep a look out for these 2 if performance starts throttling. Also, app.yaml and cm.yaml can be found in the /deploy folder while cache.yaml and redis.yaml can be found in the /scripts/setup folder. 

Though this is less important, the populate.py scripts found in some benchmarks can also stall. Thus, you can increase the number of workers and add a retry mechanic like I did to ensure populate.py doesn't stall half way.

### General Codebase layout
Here, we quickly explain what each folder in the MuCache codebase contains:
- cmd: contains main.go files for each service that hooks up the ports and endpoints for the service. Also used to initialize the cache via a zmqproxy server for each service. I personally used this to hook up prefetching-specific functions to the endpoint
- deploy: contains configuration files and Dockerfiles for the app services and cache managers. These are utilized whenever scripts/deploy.sh is called.
- experiments: contains the main experiments and benchmarks used to evaluate MuCache. Each experiment has a run.py file to run the experiment along with additional code for supplementary things. One thing to note is that they all utilize /experiments/helper.py to call common functions such as deploy() and clean2(). So any modifications to setup/deployment should also be changed in helper.py.
- internal: contains the internal code and logic for the app services.
- pkg: contains the code for MuCache and its related components such as wrappers. Can customize or add additional logic to MuCache here.
- playground: contains old legacy code to run scripts and tests. Unsure if it works. Not used in experiments.
- proxy: contains all the code for the proxy server. Each experiment has its own rust file that dictates how requests are created and sent to the microservice. You can add new/modified workloads here like I did for prefetching and batched invalidation. However, make sure to import it into main.rs to have it work.
- results: contains all the results obtained from MY experiments (original MuCache doesn't have this). Main folder to check is paper_results, which was used to generate the results for my paper.
- scripts: contains the setup scripts to deploy everything from the Kubernetes cluster to the app service pods. When a script is called from experiments/helper.py, it is usually from this folder. Thus, when modifying MuCache, you should pay attention to the scripts folder as well
- tests: contains small tests for the real-world app benchmarks. Didn't run for me.

Overall, the main folders to consider when exploring MuCache is the cmd, deploy, experiments, proxy, scripts, internal, and pkg folders. As a side note, there is still a lot of legacy code within these folders, so make sure to see if you're analyzing the right thing.



