package main

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

// computeStats summarises samples in milliseconds. Percentiles use linear
// interpolation between closest ranks, so p99 of 200 samples is not simply
// the second-largest value.
func computeStats(samples []float64) *Stats {
	if len(samples) == 0 {
		return nil
	}
	s := append([]float64(nil), samples...)
	sort.Float64s(s)
	var sum float64
	for _, v := range s {
		sum += v
	}
	return &Stats{
		Count:  len(s),
		Min:    round2(s[0]),
		Median: round2(percentile(s, 50)),
		Mean:   round2(sum / float64(len(s))),
		P95:    round2(percentile(s, 95)),
		P99:    round2(percentile(s, 99)),
		Max:    round2(s[len(s)-1]),
	}
}

// percentile expects sorted input.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := p / 100 * float64(len(sorted)-1)
	lo := int(rank)
	if lo >= len(sorted)-1 {
		return sorted[len(sorted)-1]
	}
	frac := rank - float64(lo)
	return sorted[lo] + frac*(sorted[lo+1]-sorted[lo])
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	s := append([]float64(nil), values...)
	sort.Float64s(s)
	return percentile(s, 50)
}

func detectEnv(cpuOverride string) Env {
	env := Env{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		NumCPU:    runtime.NumCPU(),
		GoVersion: runtime.Version(),
		CPU:       cpuOverride,
	}
	if env.CPU == "" {
		env.CPU = cpuModel()
	}
	env.OSVersion = osVersion()
	return env
}

func cpuModel() string {
	switch runtime.GOOS {
	case "linux":
		f, err := os.Open("/proc/cpuinfo")
		if err != nil {
			return ""
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if name, value, ok := strings.Cut(sc.Text(), ":"); ok && strings.TrimSpace(name) == "model name" {
				return strings.TrimSpace(value)
			}
		}
	case "darwin":
		return command("sysctl", "-n", "machdep.cpu.brand_string")
	case "windows":
		if v := command("powershell", "-NoProfile", "-Command", "(Get-CimInstance Win32_Processor).Name"); v != "" {
			return v
		}
	}
	return ""
}

func osVersion() string {
	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return ""
		}
		for _, line := range strings.Split(string(data), "\n") {
			if v, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
				return strings.Trim(v, `"`)
			}
		}
	case "darwin":
		return "macOS " + command("sw_vers", "-productVersion")
	case "windows":
		return command("cmd", "/c", "ver")
	}
	return ""
}

func command(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
