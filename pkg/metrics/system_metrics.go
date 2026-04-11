package metrics

import (
	"context"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

var (
	// System-level metrics
	systemCPUPercent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_cpu_percent",
		Help: "System CPU usage percentage",
	})

	systemMemoryUsed = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_memory_used_bytes",
		Help: "System memory used in bytes",
	})

	systemMemoryTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_memory_total_bytes",
		Help: "System total memory in bytes",
	})

	systemMemoryPercent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_memory_percent",
		Help: "System memory usage percentage",
	})

	systemLoadAvg1 = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_load_avg_1min",
		Help: "System load average (1 minute)",
	})

	systemLoadAvg5 = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_load_avg_5min",
		Help: "System load average (5 minutes)",
	})

	systemLoadAvg15 = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_load_avg_15min",
		Help: "System load average (15 minutes)",
	})

	systemNetConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_net_connections_total",
		Help: "Total number of network connections",
	})

	// Process-level metrics
	processCPUPercent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "process_cpu_percent",
		Help: "Current process CPU usage percentage",
	})

	processMemoryRSS = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "process_memory_rss_bytes",
		Help: "Current process memory RSS in bytes",
	})

	processConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "process_connections_total",
		Help: "Current process number of connections",
	})
)

// StartSystemMetricsCollector starts a background goroutine that periodically
// collects system and process metrics for Prometheus.
// The goroutine will stop gracefully when the context is cancelled.
func StartSystemMetricsCollector(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Get current process for process-level metrics
		proc, err := process.NewProcess(int32(os.Getpid()))
		if err != nil {
			log.Warn().Err(err).Msg("failed to get current process for metrics")
		}

		// Collect initial metrics immediately
		collectSystemMetrics()
		if proc != nil {
			collectProcessMetrics(proc)
		}

		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("system metrics collector stopped")
				return
			case <-ticker.C:
				collectSystemMetrics()
				if proc != nil {
					collectProcessMetrics(proc)
				}
			}
		}
	}()
}

func collectSystemMetrics() {
	// CPU usage
	if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
		systemCPUPercent.Set(cpuPercent[0])
	}

	// Memory usage
	if vmStat, err := mem.VirtualMemory(); err == nil {
		systemMemoryUsed.Set(float64(vmStat.Used))
		systemMemoryTotal.Set(float64(vmStat.Total))
		systemMemoryPercent.Set(vmStat.UsedPercent)
	}

	// Load average
	if loadStat, err := load.Avg(); err == nil {
		systemLoadAvg1.Set(loadStat.Load1)
		systemLoadAvg5.Set(loadStat.Load5)
		systemLoadAvg15.Set(loadStat.Load15)
	}

	// Network connections
	if conns, err := net.Connections("all"); err == nil {
		systemNetConnections.Set(float64(len(conns)))
	}
}

func collectProcessMetrics(proc *process.Process) {
	// Process CPU usage
	if cpuPercent, err := proc.CPUPercent(); err == nil {
		processCPUPercent.Set(cpuPercent)
	}

	// Process memory usage
	if memInfo, err := proc.MemoryInfo(); err == nil {
		processMemoryRSS.Set(float64(memInfo.RSS))
	}

	// Process connections
	if conns, err := proc.Connections(); err == nil {
		processConnections.Set(float64(len(conns)))
	}
}
