#!/usr/bin/env python3
import signal
import time
from experiments.helper import *
from pprint import pprint

APP = "boutique"
set_app(APP)

## Increase this to increase the latency of home requests (10% of requests)
CATALOG_SIZE = 10

## Tweaking this will vary how of1ten the cart of a user is accessed
USERS = 10000

## Tweaking this will vary how many objects there are and their size
PRODUCTS = 10000
PRODUCT_SIZE = 5000

## Tweak the cache size for the hit ratio (in MB)
CACHE_SIZE = "80"
# CACHE_SIZE = "0"

## Tweaking the TTL affects the TTL baseline (in ms)
TTL = "10"


def start_proxy(workload = ""):
    global CATALOG_SIZE
    run_shell("cd proxy && cargo build --release")
    frontend_ip = get_ip("frontend")
    cart_ip = get_ip("cart")
    currency_ip = get_ip("currency")

    print( "Running workload:", workload )
    if( workload == "nearby_requests" ):
        p = run_in_bg(
        f"cargo run --release boutique-prefetch --frontend {frontend_ip} --cart {cart_ip} --currency {currency_ip} --catalog-size {CATALOG_SIZE}",
        "proxy")
    elif( workload == "write_heavy" ):
        p = run_in_bg(
        f"cargo run --release boutique-write-heavy --frontend {frontend_ip} --cart {cart_ip} --currency {currency_ip} --catalog-size {CATALOG_SIZE}",
        "proxy")
    elif( workload == "write_heavy_hotkeys"):
        p = run_in_bg(
        f"cargo run --release boutique-write-heavy-hot-keys --frontend {frontend_ip} --cart {cart_ip} --currency {currency_ip} --catalog-size {CATALOG_SIZE}",
        "proxy")
    else:
        p = run_in_bg(
            f"cargo run --release boutique --frontend {frontend_ip} --cart {cart_ip} --currency {currency_ip} --catalog-size {CATALOG_SIZE}",
            "proxy")
    time.sleep(5)
    return p


def populate():
    global USERS, PRODUCTS, PRODUCT_SIZE
    args = ""
    args += f" --users {USERS}"
    args += f" --products {PRODUCTS}"
    args += f" --product_size {PRODUCT_SIZE}"
    for service in ["frontend", "product_catalog", "currency"]:
        ip = get_ip(service)
        args += f" --{service} {ip}"
    run_shell("python3 experiments/boutique/populate.py" + args)


def run_once(req: int, cm: str, ttl=None, batch_inval=False, prefetch_strategy = False, workload = "" ):
    global CACHE_SIZE
    clean2(mem=CACHE_SIZE)
    deploy(cm=cm, ttl=ttl, batch_inval=batch_inval, prefetch = prefetch_strategy)
    populate()
    p = start_proxy(workload)
    #top_p, top_q = top_process()
    res = run_shell(compose_oha_proxy(req=req, duration=120))
    res = parse_res(res)
    os.kill(p.pid, signal.SIGINT)
    p.terminate()
    p.wait()
    if cm in ["true", "upper"]:
        res["hit_rate"] = get_hit_rate_redis()
    # usage = json.loads(top_q.get())
    # pprint(usage)
    # top_p.join()
    return res


def run_resource_usage():
    reqs = 3500
    res = run_once(reqs, cm="true")
    print(res['raw'])
    del res['raw']
    pprint(res)


def main():
    reqs = [2000, 3000, 4000, 4500, 5000, 5500, 6000]
    #reqs = [3000]
    ttl = TTL  ## in ms
    baselines = {}
    uppers = {}
    ours = {}
    batch_inval = {}
    prefetched = {}

    ## Note: Save every iteration so that we get incremental results
    #file_ext = "-nearbyUsers-halfProductSize-5000app-12proxy-4zmq"
    #file_ext = "-test-nearby-1neighbor-originalContextWith10msTimeOut"
    # file_ext = "-nearbyUsers-2neighbor-originalContextWith10msTimeOut-halfProductSize-5000app-12proxy-1zmq-async"
    file_ext = "-halfProductSize-5000app-12proxy-1zmq"
    for req in reqs:
        # baseline = run_once(req, cm="false", workload="write_heavy_hotkeys")
        # baselines[req] = baseline
        # with open(f"{APP}-baseline{file_ext}.json", "w") as f:
        #     json.dump(baselines, f, indent=2)

        # upper = run_once(req, cm="upper")
        # uppers[req] = upper
        # with open(f"{APP}-upper{file_ext}.json", "w") as f:
        #     json.dump(uppers, f, indent=2)

        # batch = run_once(req, cm="true", batch_inval=True) #, workload="write_heavy_hotkeys")
        # batch_inval[req] = batch
        # with open(f"{APP}-batch{file_ext}.json", "w") as f:
        #     json.dump(batch_inval, f, indent=2)

        # our = run_once(req, cm="true", workload="write_heavy_hotkeys")
        # ours[req] = our
        # with open(f"{APP}{file_ext}.json", "w") as f:
        #     json.dump(ours, f, indent=2)

        prefetch = run_once(req, cm="true", prefetch_strategy=True) #, nearby_requests=True)
        prefetched[req] = prefetch
        with open(f"{APP}-prefetch{file_ext}.json", "w") as f:
            json.dump(prefetched, f, indent=2)
    clean2()

    print(baselines)
    print(ours)
    print(uppers)

if __name__ == "__main__":
    main()
