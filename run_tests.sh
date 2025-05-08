#!/bin/bash -ex
# python3 experiments/star/run.py  # fanout
# python3 experiments/fanin/run.py # fanin
# python3 experiments/twoservices/hitrate.py
python3 experiments/movie/run.py
python3 experiments/boutique/run.py
python3 experiments/social/run.py
python3 experiments/hotel/run.py
# python3 experiments/cachesize/run.py
#python3 experiments/hotel/ttl.py