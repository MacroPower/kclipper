| Command                                  |  Mean [ms] | Min [ms] | Max [ms] |    Relative |
| :--------------------------------------- | ---------: | -------: | -------: | ----------: |
| `.tmp/bin/kcl ./benchmarks/simple.k`     | 24.6 ± 3.0 |     22.5 |     51.9 |        1.00 |
| `kclx ./benchmarks/simple.k`             | 36.8 ± 0.7 |     34.7 |     38.5 | 1.50 ± 0.18 |
| `kclx ./benchmarks/simple-helm.k`        | 40.8 ± 3.1 |     37.7 |     63.9 | 1.66 ± 0.24 |
| `kclx ./benchmarks/simple-helm-values.k` | 42.7 ± 0.9 |     40.2 |     45.0 | 1.73 ± 0.21 |
