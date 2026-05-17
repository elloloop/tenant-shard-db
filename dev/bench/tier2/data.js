window.BENCHMARK_DATA = {
  "lastUpdate": 1778977783617,
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
        "date": 1778976785932,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2332.5457288884522,
            "unit": "iter/sec",
            "range": "stddev: 0.00003223363202096383",
            "extra": "mean: 428.7161394587271 usec\nrounds: 1219"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1663.7058834677086,
            "unit": "iter/sec",
            "range": "stddev: 0.000053223188433165015",
            "extra": "mean: 601.0677788285944 usec\nrounds: 1058"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 824.4005595738923,
            "unit": "iter/sec",
            "range": "stddev: 0.0005251918185447465",
            "extra": "mean: 1.2130025730657799 msec\nrounds: 698"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 748.4048135570138,
            "unit": "iter/sec",
            "range": "stddev: 0.00012852822998794962",
            "extra": "mean: 1.3361752648906762 msec\nrounds: 638"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1186.8472106510303,
            "unit": "iter/sec",
            "range": "stddev: 0.0018240905328235826",
            "extra": "mean: 842.5684376436815 usec\nrounds: 1307"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1101.7208370363228,
            "unit": "iter/sec",
            "range": "stddev: 0.002408550506953024",
            "extra": "mean: 907.6709511005018 usec\nrounds: 1227"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1221.7320251136225,
            "unit": "iter/sec",
            "range": "stddev: 0.001621203101784329",
            "extra": "mean: 818.5100983229107 usec\nrounds: 1312"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1605.9848473268767,
            "unit": "iter/sec",
            "range": "stddev: 0.00003856268083331603",
            "extra": "mean: 622.6708811508876 usec\nrounds: 1321"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1472.8515314187578,
            "unit": "iter/sec",
            "range": "stddev: 0.00006280466293332633",
            "extra": "mean: 678.9550600777305 usec\nrounds: 516"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1308.9741096403554,
            "unit": "iter/sec",
            "range": "stddev: 0.000043033250511987144",
            "extra": "mean: 763.9570505139731 usec\nrounds: 1069"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 1913.7718723023613,
            "unit": "iter/sec",
            "range": "stddev: 0.00002910580698385685",
            "extra": "mean: 522.5283193220679 usec\nrounds: 1475"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 182.8828916410737,
            "unit": "iter/sec",
            "range": "stddev: 0.000935151374642914",
            "extra": "mean: 5.467980033707045 msec\nrounds: 178"
          }
        ]
      },
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
        "date": 1778977783320,
        "tool": "pytest",
        "benches": [
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_health",
            "value": 2512.3262022963686,
            "unit": "iter/sec",
            "range": "stddev: 0.0000276893543337289",
            "extra": "mean: 398.0374837813494 usec\nrounds: 1079"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_node",
            "value": 1763.7797425581098,
            "unit": "iter/sec",
            "range": "stddev: 0.00005888943032124892",
            "extra": "mean: 566.9642166031703 usec\nrounds: 1048"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_nodes_batch",
            "value": 961.3991305639422,
            "unit": "iter/sec",
            "range": "stddev: 0.0003060729665692973",
            "extra": "mean: 1.0401507222223252 msec\nrounds: 792"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_query_nodes",
            "value": 737.8064681235234,
            "unit": "iter/sec",
            "range": "stddev: 0.00007882924030089977",
            "extra": "mean: 1.3553689798129829 msec\nrounds: 644"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node",
            "value": 1216.1362055289424,
            "unit": "iter/sec",
            "range": "stddev: 0.0019092160983738957",
            "extra": "mean: 822.2763169566712 usec\nrounds: 1262"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_create_node_and_edge",
            "value": 1143.303759252838,
            "unit": "iter/sec",
            "range": "stddev: 0.0023822986359390867",
            "extra": "mean: 874.6581928966203 usec\nrounds: 1633"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_execute_atomic_update_node",
            "value": 1240.575786316372,
            "unit": "iter/sec",
            "range": "stddev: 0.0019267477977788507",
            "extra": "mean: 806.0773158964266 usec\nrounds: 1472"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_from",
            "value": 1676.3100532719268,
            "unit": "iter/sec",
            "range": "stddev: 0.00004485731043698995",
            "extra": "mean: 596.5483521667949 usec\nrounds: 1292"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_edges_to",
            "value": 1467.3434201112552,
            "unit": "iter/sec",
            "range": "stddev: 0.000026989558988029454",
            "extra": "mean: 681.5037204611441 usec\nrounds: 347"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_get_connected_nodes",
            "value": 1281.5385238418282,
            "unit": "iter/sec",
            "range": "stddev: 0.00006463596371799978",
            "extra": "mean: 780.312086914231 usec\nrounds: 1024"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_search_nodes",
            "value": 2085.005974066773,
            "unit": "iter/sec",
            "range": "stddev: 0.00002675100074484585",
            "extra": "mean: 479.61493273302943 usec\nrounds: 1665"
          },
          {
            "name": "tests/python/benchmarks/bench_entdb.py::test_entdb_mailbox_like_list",
            "value": 170.70669688749686,
            "unit": "iter/sec",
            "range": "stddev: 0.00045596845144396496",
            "extra": "mean: 5.858000993710536 msec\nrounds: 159"
          }
        ]
      }
    ]
  }
}