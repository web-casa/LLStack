package site

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// AccessStats captures parsed access log statistics.
type AccessStats struct {
	Site          string            `json:"site"`
	LogPath       string            `json:"log_path"`
	TotalRequests int               `json:"total_requests"`
	TopURLs       []StatEntry       `json:"top_urls"`
	TopIPs        []StatEntry       `json:"top_ips"`
	StatusCodes   map[string]int    `json:"status_codes"`
	Methods       map[string]int    `json:"methods"`
}

// StatEntry is a ranked item with count.
type StatEntry struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// Combined log format regex:
// 127.0.0.1 - - [27/Mar/2026:00:31:21 +0000] "GET /index.php HTTP/1.1" 200 1234
var logLinePattern = regexp.MustCompile(`^(\S+)\s+\S+\s+\S+\s+\[.*?\]\s+"(\S+)\s+(\S+)\s+\S+"\s+(\d{3})`)

// AnalyzeAccessLog parses an Apache/OLS/LSWS combined access log and returns statistics.
func AnalyzeAccessLog(siteName, logPath string, topN int) (AccessStats, error) {
	stats := AccessStats{
		Site:        siteName,
		LogPath:     logPath,
		StatusCodes: map[string]int{},
		Methods:     map[string]int{},
	}

	if topN <= 0 {
		topN = 10
	}

	file, err := os.Open(logPath)
	if err != nil {
		return stats, fmt.Errorf("open access log: %w", err)
	}
	defer file.Close()

	urlCounts := map[string]int{}
	ipCounts := map[string]int{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		matches := logLinePattern.FindStringSubmatch(line)
		if len(matches) < 5 {
			continue
		}
		ip := matches[1]
		method := matches[2]
		url := matches[3]
		status := matches[4]

		stats.TotalRequests++
		ipCounts[ip]++
		urlCounts[url]++
		stats.StatusCodes[status]++
		stats.Methods[method]++
	}

	stats.TopURLs = topEntries(urlCounts, topN)
	stats.TopIPs = topEntries(ipCounts, topN)

	return stats, nil
}

// StatsText returns a human-readable stats summary.
func StatsText(s AccessStats) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Site: %s\nLog: %s\nTotal requests: %d\n", s.Site, s.LogPath, s.TotalRequests)

	fmt.Fprintf(&b, "\nStatus Codes:\n")
	for code, count := range s.StatusCodes {
		fmt.Fprintf(&b, "  %s: %d\n", code, count)
	}

	fmt.Fprintf(&b, "\nTop URLs:\n")
	for _, e := range s.TopURLs {
		fmt.Fprintf(&b, "  %-50s %d\n", e.Key, e.Count)
	}

	fmt.Fprintf(&b, "\nTop IPs:\n")
	for _, e := range s.TopIPs {
		fmt.Fprintf(&b, "  %-20s %d\n", e.Key, e.Count)
	}

	return b.String()
}

func topEntries(counts map[string]int, n int) []StatEntry {
	entries := make([]StatEntry, 0, len(counts))
	for k, v := range counts {
		entries = append(entries, StatEntry{Key: k, Count: v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Count > entries[j].Count
	})
	if len(entries) > n {
		entries = entries[:n]
	}
	return entries
}
