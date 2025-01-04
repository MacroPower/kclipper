| Command                                         |  Mean [ms] | Min [ms] | Max [ms] |    Relative |
| :---------------------------------------------- | ---------: | -------: | -------: | ----------: |
| `.tmp/bin/kcl ./benchmarks/no-charts.k`         | 46.0 ± 2.4 |     43.5 |     82.7 |        1.00 |
| `kclipper ./benchmarks/no-charts.k`             | 58.5 ± 2.6 |     56.2 |     94.5 | 1.27 ± 0.09 |
| `kclipper ./benchmarks/10-charts.k`             | 80.4 ± 2.9 |     77.3 |    116.3 | 1.75 ± 0.11 |
| `kclipper ./benchmarks/10-charts-with-values.k` | 87.8 ± 3.5 |     84.0 |    126.6 | 1.91 ± 0.12 |
