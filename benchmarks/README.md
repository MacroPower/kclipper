| Command                                  |  Mean [ms] | Min [ms] | Max [ms] |    Relative |
| :--------------------------------------- | ---------: | -------: | -------: | ----------: |
| `.tmp/bin/kcl ./benchmarks/simple.k`     | 22.4 ± 0.9 |     20.6 |     25.7 |        1.00 |
| `kclx ./benchmarks/simple.k`             | 26.6 ± 0.7 |     25.0 |     28.7 | 1.19 ± 0.06 |
| `kclx ./benchmarks/simple-helm.k`        | 66.9 ± 0.9 |     65.3 |     70.1 | 2.98 ± 0.12 |
| `kclx ./benchmarks/simple-helm-values.k` | 70.0 ± 1.6 |     67.4 |     77.5 | 3.12 ± 0.14 |
