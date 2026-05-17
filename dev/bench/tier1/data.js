window.BENCHMARK_DATA = {
  "lastUpdate": 1778977315692,
  "repoUrl": "https://github.com/elloloop/tenant-shard-db",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "email": "arun88m@gmail.com",
            "name": "Arun Saragadam",
            "username": "iarunsaragadam"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "75d6e712283d730f6670b82936609fbb0859f11a",
          "message": "Fix schemaless CAS preconditions\n\nFixes #525",
          "timestamp": "2026-05-17T00:43:26+01:00",
          "tree_id": "b4192c1e861c0b8afda9c97d86893bca565d6ea8",
          "url": "https://github.com/elloloop/tenant-shard-db/commit/75d6e712283d730f6670b82936609fbb0859f11a"
        },
        "date": 1778977315383,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 3387.999700029148,
            "unit": "iter/sec",
            "range": "stddev: 0.000025369112531249446",
            "extra": "mean: 295.1594122016589 usec\nrounds: 1344"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 2219.322244088495,
            "unit": "iter/sec",
            "range": "stddev: 0.00003514310524790546",
            "extra": "mean: 450.5880129231585 usec\nrounds: 1393"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 1105.3564622296715,
            "unit": "iter/sec",
            "range": "stddev: 0.00008511092165527612",
            "extra": "mean: 904.6855328306025 usec\nrounds: 929"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 814.5844027181297,
            "unit": "iter/sec",
            "range": "stddev: 0.0000811953608881937",
            "extra": "mean: 1.2276198717568982 msec\nrounds: 655"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1968.3799540046336,
            "unit": "iter/sec",
            "range": "stddev: 0.00007619280849049144",
            "extra": "mean: 508.0319975650626 usec\nrounds: 1642"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1971.2150021878338,
            "unit": "iter/sec",
            "range": "stddev: 0.00007743166419283495",
            "extra": "mean: 507.3013338931111 usec\nrounds: 1785"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1863.5156200669217,
            "unit": "iter/sec",
            "range": "stddev: 0.00014300862060160163",
            "extra": "mean: 536.6201330601612 usec\nrounds: 1706"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1988.9872542271678,
            "unit": "iter/sec",
            "range": "stddev: 0.000060453492631752935",
            "extra": "mean: 502.7684304535957 usec\nrounds: 1366"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1664.695481528081,
            "unit": "iter/sec",
            "range": "stddev: 0.00008513181571861864",
            "extra": "mean: 600.7104669269996 usec\nrounds: 257"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1434.6063642525223,
            "unit": "iter/sec",
            "range": "stddev: 0.0000600204354602292",
            "extra": "mean: 697.0553211793629 usec\nrounds: 1152"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2666.644332669093,
            "unit": "iter/sec",
            "range": "stddev: 0.00002970117680495202",
            "extra": "mean: 375.00314074471333 usec\nrounds: 1961"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 152.3238984292626,
            "unit": "iter/sec",
            "range": "stddev: 0.00014173186198351557",
            "extra": "mean: 6.564958028988394 msec\nrounds: 138"
          }
        ]
      }
    ]
  }
}