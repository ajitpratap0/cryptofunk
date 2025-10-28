# Performance Baselines for MockOrchestrator

**Test Environment:**
- Platform: darwin/arm64
- CPU: Apple M4 Max
- Go Version: 1.21+
- NATS: localhost:4222
- Date: 2025-10-28

## Benchmark Results

### 1. BenchmarkMockOrchestrator_SignalProcessing
**Measures:** Signal processing latency through NATS pub/sub

- **Operations/sec:** 5,577,174 iterations
- **Latency:** 22,747 ns/op (~22.7 µs)
- **Memory:** 83,826 B/op
- **Allocations:** 1 allocs/op

**Analysis:** Signal processing is highly efficient at ~23µs per signal. Single allocation suggests good memory efficiency.

### 2. BenchmarkMockOrchestrator_DecisionMaking
**Measures:** Decision-making latency including reset and signal wait

- **Operations/sec:** 351 iterations
- **Latency:** 10,258,894 ns/op (~10.26 ms)
- **Memory:** 1,013 B/op
- **Allocations:** 17 allocs/op

**Analysis:** Decision-making takes ~10ms per cycle. This includes orchestrator reset, signal publish, and WaitForDecision(5s timeout). The high latency is expected due to synchronization overhead.

### 3. BenchmarkMockOrchestrator_MultipleSignals
**Measures:** Performance with 3 simultaneous signals from different sources

- **Operations/sec:** 348 iterations
- **Latency:** 10,247,629 ns/op (~10.25 ms)
- **Memory:** 3,877 B/op
- **Allocations:** 54 allocs/op

**Analysis:** Multi-signal aggregation is comparable to single-signal decision-making (~10ms). Higher allocations (54 vs 17) reflect handling multiple signal sources.

### 4. BenchmarkMockOrchestrator_ConcurrentSignals
**Measures:** Concurrent signal handling performance using RunParallel

- **Operations/sec:** 3,539,805 iterations
- **Latency:** 3,872 ns/op (~3.9 µs)
- **Memory:** 7,972 B/op
- **Allocations:** 2 allocs/op

**Analysis:** Concurrent signal publishing is extremely fast at ~4µs per operation. The orchestrator handles concurrent load well with minimal allocations.

### 5. BenchmarkDefaultDecisionPolicy
**Measures:** Pure decision policy algorithm performance (no I/O)

- **Operations/sec:** 7,685,312 iterations
- **Latency:** 433.1 ns/op (~0.43 µs)
- **Memory:** 512 B/op
- **Allocations:** 7 allocs/op

**Analysis:** Decision policy computation is highly optimized at sub-microsecond latency. This represents the core weighted voting algorithm without any NATS overhead.

### 6. BenchmarkMockOrchestrator_GettersUnderLoad
**Measures:** Getter method performance under concurrent access

- **Operations/sec:** 702,820 iterations
- **Latency:** 4,904 ns/op (~4.9 µs)
- **Memory:** 19,264 B/op
- **Allocations:** 4 allocs/op

**Analysis:** Thread-safe getter methods perform well at ~5µs per operation. Each operation calls 5 getters (GetReceivedSignals, GetDecisions, GetSignalCount, GetLastSignal, GetLastDecision), so individual getter latency is ~1µs.

## Performance Summary

**Key Metrics:**
- **Signal Processing:** ~23 µs per signal
- **Decision Making:** ~10 ms (includes synchronization)
- **Decision Algorithm:** ~0.43 µs (pure computation)
- **Concurrent Signals:** ~4 µs per operation
- **Getter Methods:** ~1 µs per getter

**Bottlenecks Identified:**
1. Decision-making latency (~10ms) is dominated by synchronization/waiting, not computation
2. Memory allocations are minimal across all benchmarks
3. No obvious performance bottlenecks detected

**Recommendations:**
1. Current performance is excellent for real-time trading scenarios
2. Signal processing at 23µs allows handling 43,000+ signals/second
3. Decision-making at 10ms supports 100 decisions/second, adequate for trading
4. Consider async decision-making if higher throughput needed

## Test Duration
Total benchmark time: 193.193s

All benchmarks: **PASS**
