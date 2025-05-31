#!/usr/bin/env python3
import signal
import time
from experiments.helper import *
from pprint import pprint

APP = "movie"
set_app(APP)


def start_proxy(workload = ""):
    run_shell("cd proxy && cargo build --release")
    frontend_ip = get_ip("frontend")
    page_ip = get_ip("page")
    if( workload == "nearby_requests" ):
        p = run_in_bg(
        f"cargo run --release movie-prefetch --frontend {frontend_ip} --page {page_ip}",
        "proxy")
    elif( workload == "write_heavy" ):
        p = run_in_bg(
        f"cargo run --release movie-write-heavy --frontend {frontend_ip} --page {page_ip}",
        "proxy")
    elif( workload == "write_heavy_hotkeys" ):
        p = run_in_bg(
        f"cargo run --release movie-write-heavy-hot-keys --frontend {frontend_ip} --page {page_ip}",
        "proxy")
    else:
        p = run_in_bg(
            f"cargo run --release movie --frontend {frontend_ip} --page {page_ip}",
            "proxy")
    time.sleep(5)
    return p


def populate():
    args = ""
    for service in APPS[APP]:
        ip = get_ip(service)
        args += f" --{service} {ip}"
    run_shell("python3 experiments/movie/populate.py" + args + " > out.txt" )


def run_once(req: int, cm: str, ttl=None, batch_inval=False,prefetch_strategy = False, workload = ""):
    clean2(mem="70")
    deploy(cm=cm, ttl=ttl, batch_inval=batch_inval, prefetch=prefetch_strategy)
    populate()
    p = start_proxy(workload)
    # top_p, top_q = top_process()
    res = run_shell(compose_oha_proxy(req=req, duration = 120))
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
    req = 2500
    res = run_once(req, cm="true")
    with open("res.json", "w") as f:
        json.dump(res, f, indent=2)
    del res["raw"]
    pprint(res)


def main():
    reqs = [1000, 1500, 2000, 2500, 3000, 3500, 4000]
    # reqs = [2500]
    ttl = "6"  ## in ms
    baselines = {}
    uppers = {}
    ours = {}
    batch_inval = {}
    prefetched = {}

    # To run different experiments
    file_ext = "-5000app-12proxy-1zmq"
    for req in reqs:
        # baseline = run_once(req, cm="false", workload="write_heavy_hotkeys")
        # baselines[req] = baseline
        # with open(f"{APP}-baseline{file_ext}.json", "w") as f:
        #     json.dump(baselines, f, indent=2)

        # upper = run_once(req, cm="upper")
        # uppers[req] = upper
        # with open(f"{APP}-upper{file_ext}.json", "w") as f:
        #     json.dump(uppers, f, indent=2)

        # our = run_once(req, cm="true", workload="write_heavy_hotkeys")
        # ours[req] = our
        # with open(f"{APP}{file_ext}.json", "w") as f:
        #     json.dump(ours, f, indent=2)

        # batch = run_once(req, cm="true", batch_inval = True, workload="write_heavy_hotkeys" )
        # batch_inval[req] = batch
        # with open(f"{APP}-batch{file_ext}.json", "w") as f:
        #     json.dump(batch_inval, f, indent=2)

        prefetch = run_once(req, cm="true", prefetch_strategy=True) #, nearby_requests=True)
        prefetched[req] = prefetch
        with open(f"{APP}-prefetch{file_ext}.json", "w") as f:
            json.dump(prefetched, f, indent=2)
    clean2()

    print(baselines)
    print(ours)
    print(uppers)
    # with open(f"{APP}-baseline{file_ext}.json", "w") as f:
    #     json.dump(baselines, f, indent=2)
    # with open(f"{APP}{file_ext}.json", "w") as f:
    #     json.dump(ours, f, indent=2)
    # with open(f"{APP}-upper{file_ext}.json", "w") as f:
    #     json.dump(uppers, f, indent=2)
    # with open(f"{APP}-batch{file_ext}.json", "w") as f:
    #     json.dump(batch_inval, f, indent=2)
    with open(f"{APP}-prefetch{file_ext}.json", "w") as f:
        json.dump(prefetched, f, indent=2)


if __name__ == "__main__":
    main()
