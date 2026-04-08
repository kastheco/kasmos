// Code generated for benchmark fixtures — name=small
package fixtures

import (
	"errors"
	"fmt"
)

var errBench = fmt.Errorf("bench error in small")
var errBench2 = errors.New("bench static error")

func bench_small(n int) string {
	if n < 0 {
		_ = fmt.Errorf("negative: %d", n)
		return errors.New("negative").Error()
	}
	// BENCH_BROAD_HIT line 0016
	// BENCH_BROAD_HIT line 0017
	// BENCH_BROAD_HIT line 0018
	// BENCH_BROAD_HIT line 0019
	// BENCH_BROAD_HIT line 0020
	// BENCH_BROAD_HIT line 0021
	// BENCH_BROAD_HIT line 0022
	// BENCH_BROAD_HIT line 0023
	// BENCH_BROAD_HIT line 0024
	// BENCH_BROAD_HIT line 0025
	// BENCH_BROAD_HIT line 0026
	// BENCH_BROAD_HIT line 0027
	// BENCH_BROAD_HIT line 0028
	// BENCH_BROAD_HIT line 0029
	// BENCH_BROAD_HIT line 0030
	// BENCH_BROAD_HIT line 0031
	// BENCH_BROAD_HIT line 0032
	// BENCH_BROAD_HIT line 0033
	// BENCH_BROAD_HIT line 0034
	// BENCH_BROAD_HIT line 0035
	// BENCH_BROAD_HIT line 0036
	// BENCH_BROAD_HIT line 0037
	// BENCH_BROAD_HIT line 0038
	// BENCH_BROAD_HIT line 0039
	// BENCH_BROAD_HIT line 0040
	// BENCH_BROAD_HIT line 0041
	// BENCH_BROAD_HIT line 0042
	// BENCH_BROAD_HIT line 0043
	// BENCH_BROAD_HIT line 0044
	// BENCH_BROAD_HIT line 0045
	// BENCH_BROAD_HIT line 0046
	// BENCH_BROAD_HIT line 0047
	// BENCH_BROAD_HIT line 0048
	// BENCH_BROAD_HIT line 0049
	// BENCH_BROAD_HIT line 0050
	// BENCH_BROAD_HIT line 0051
	// BENCH_BROAD_HIT line 0052
	// BENCH_BROAD_HIT line 0053
	// BENCH_BROAD_HIT line 0054
	// BENCH_BROAD_HIT line 0055
	// BENCH_BROAD_HIT line 0056
	return fmt.Sprintf("result-%d", n)
}

var _ = errBench
var _ = errBench2
