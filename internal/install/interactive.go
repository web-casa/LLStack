package install

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/web-casa/llstack/internal/system"
)

// InteractiveResult captures the user's choices from the interactive wizard.
type InteractiveResult struct {
	Backend       string
	PHPVersion    string
	ExtraPHP      []string
	DBProvider    string
	DBRootPass    string
	DBPassAuto    bool
	CacheProvider string // redis / memcached / valkey / redis+memcached / ""
	WithRedis     bool
	WithMemcached bool
	WithValkey    bool
	EnableFail2ban bool
	EnableTuning   bool
	Confirmed     bool
}

// RunInteractive executes the interactive install wizard.
func RunInteractive(stdin io.Reader, stdout io.Writer) (InteractiveResult, error) {
	reader := bufio.NewReader(stdin)
	result := InteractiveResult{}

	hw := system.DetectHardware()
	osInfo := detectOSName()

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "╔══════════════════════════════════════╗")
	fmt.Fprintln(stdout, "║     LLStack Web Stack Installer      ║")
	fmt.Fprintln(stdout, "║     EL9 / EL10 · CLI + TUI          ║")
	fmt.Fprintln(stdout, "╚══════════════════════════════════════╝")
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "Detected: %s · %d CPU · %.1f GB RAM\n\n", osInfo, hw.CPUCores, hw.MemoryGB)

	// Step 1: Backend
	fmt.Fprintln(stdout, "─── Step 1: Web Backend ───")
	fmt.Fprintln(stdout, "  1) Apache")
	fmt.Fprintln(stdout, "  2) OpenLiteSpeed")
	fmt.Fprintln(stdout, "  3) LiteSpeed Enterprise")
	choice := promptChoice(reader, stdout, "> ", 1, 3, 1)
	switch choice {
	case 1:
		result.Backend = "apache"
	case 2:
		result.Backend = "ols"
	case 3:
		result.Backend = "lsws"
	}
	fmt.Fprintln(stdout, "")

	// Step 2: PHP
	phpVersions := []string{"7.4", "8.0", "8.1", "8.2", "8.3", "8.4", "8.5"}
	eolVersions := map[string]bool{"7.4": true, "8.0": true, "8.1": true}

	fmt.Fprintln(stdout, "─── Step 2: PHP Version ───")
	for i, v := range phpVersions {
		eolMark := ""
		if eolVersions[v] {
			eolMark = " (EOL ⚠)"
		}
		fmt.Fprintf(stdout, "  %d) PHP %s%s\n", i+1, v, eolMark)
	}
	fmt.Fprintln(stdout, "  0) Skip PHP")
	phpChoice := promptChoice(reader, stdout, "> ", 0, len(phpVersions), 5) // default 8.3
	if phpChoice > 0 {
		result.PHPVersion = phpVersions[phpChoice-1]
	}

	if result.PHPVersion != "" {
		fmt.Fprint(stdout, "  Install additional PHP versions? (comma-separated, e.g. 8.2,8.4) [skip]: ")
		extra := promptLine(reader)
		if extra != "" && extra != "skip" {
			for _, v := range strings.Split(extra, ",") {
				v = strings.TrimSpace(v)
				if v != "" && v != result.PHPVersion {
					result.ExtraPHP = append(result.ExtraPHP, v)
				}
			}
		}
	}
	fmt.Fprintln(stdout, "")

	// Step 3: Database
	fmt.Fprintln(stdout, "─── Step 3: Database ───")
	fmt.Fprintln(stdout, "  1) MariaDB")
	fmt.Fprintln(stdout, "  2) MySQL")
	fmt.Fprintln(stdout, "  3) PostgreSQL")
	fmt.Fprintln(stdout, "  4) Percona Server")
	fmt.Fprintln(stdout, "  0) Skip database")
	dbChoice := promptChoice(reader, stdout, "> ", 0, 4, 1)
	switch dbChoice {
	case 1:
		result.DBProvider = "mariadb"
	case 2:
		result.DBProvider = "mysql"
	case 3:
		result.DBProvider = "postgresql"
	case 4:
		result.DBProvider = "percona"
	}

	if result.DBProvider != "" {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "  Database root password:")
		fmt.Fprintln(stdout, "  1) Set manually")
		fmt.Fprintln(stdout, "  2) Auto-generate secure password")
		passChoice := promptChoice(reader, stdout, "  > ", 1, 2, 2)
		if passChoice == 1 {
			fmt.Fprint(stdout, "  Enter root password: ")
			result.DBRootPass = promptLine(reader)
			if result.DBRootPass == "" {
				result.DBRootPass = generatePassword()
				result.DBPassAuto = true
				fmt.Fprintf(stdout, "  (empty input, auto-generated)\n")
			}
		} else {
			result.DBRootPass = generatePassword()
			result.DBPassAuto = true
		}
	}
	fmt.Fprintln(stdout, "")

	// Step 4: Cache
	fmt.Fprintln(stdout, "─── Step 4: Cache ───")
	fmt.Fprintln(stdout, "  1) Redis")
	fmt.Fprintln(stdout, "  2) Memcached")
	fmt.Fprintln(stdout, "  3) Valkey (Redis-compatible fork)")
	fmt.Fprintln(stdout, "  4) Redis + Memcached")
	fmt.Fprintln(stdout, "  0) Skip cache")
	cacheChoice := promptChoice(reader, stdout, "> ", 0, 4, 1)
	switch cacheChoice {
	case 1:
		result.WithRedis = true
	case 2:
		result.WithMemcached = true
	case 3:
		result.WithValkey = true
	case 4:
		result.WithRedis = true
		result.WithMemcached = true
	}
	fmt.Fprintln(stdout, "")

	// Step 5: Security
	fmt.Fprintln(stdout, "─── Step 5: Security ───")
	fmt.Fprintln(stdout, "  Install fail2ban for monitoring?")
	fmt.Fprintln(stdout, "  (Installed in monitor-only mode. No blocking rules by default.")
	fmt.Fprintln(stdout, "   Configure blocking via `llstack security:fail2ban` later.)")
	fmt.Fprint(stdout, "  [Y/n] ")
	result.EnableFail2ban = promptYesNo(reader, true)
	fmt.Fprintln(stdout, "")

	// Step 6: Tuning
	fmt.Fprintln(stdout, "─── Step 6: Auto-Tuning ───")
	tuningProfile := system.DetectHardware()
	fmt.Fprintf(stdout, "  Detected: %d CPU / %.1f GB RAM\n", tuningProfile.CPUCores, tuningProfile.MemoryGB)
	fmt.Fprint(stdout, "  Apply hardware-aware tuning? [Y/n] ")
	result.EnableTuning = promptYesNo(reader, true)
	fmt.Fprintln(stdout, "")

	// Review
	fmt.Fprintln(stdout, "─── Review ───")
	fmt.Fprintf(stdout, "  Backend:    %s\n", result.Backend)
	if result.PHPVersion != "" {
		extra := ""
		if len(result.ExtraPHP) > 0 {
			extra = " (+ " + strings.Join(result.ExtraPHP, ", ") + ")"
		}
		fmt.Fprintf(stdout, "  PHP:        %s%s\n", result.PHPVersion, extra)
	} else {
		fmt.Fprintln(stdout, "  PHP:        skip")
	}
	if result.DBProvider != "" {
		passHint := "(user-set)"
		if result.DBPassAuto {
			passHint = "(auto-generated)"
		}
		fmt.Fprintf(stdout, "  Database:   %s %s\n", result.DBProvider, passHint)
	} else {
		fmt.Fprintln(stdout, "  Database:   skip")
	}
	cacheDesc := "skip"
	parts := []string{}
	if result.WithRedis {
		parts = append(parts, "Redis")
	}
	if result.WithMemcached {
		parts = append(parts, "Memcached")
	}
	if result.WithValkey {
		parts = append(parts, "Valkey")
	}
	if len(parts) > 0 {
		cacheDesc = strings.Join(parts, " + ")
	}
	fmt.Fprintf(stdout, "  Cache:      %s\n", cacheDesc)
	fmt.Fprintf(stdout, "  fail2ban:   %s\n", boolYesNo(result.EnableFail2ban))
	fmt.Fprintf(stdout, "  Auto-tune:  %s\n", boolYesNo(result.EnableTuning))
	fmt.Fprintln(stdout, "  Welcome:    http://<server-ip> (with PHP probe + Adminer)")
	fmt.Fprintln(stdout, "")

	fmt.Fprint(stdout, "Proceed with installation? [Y/n] ")
	result.Confirmed = promptYesNo(reader, true)

	return result, nil
}

func promptChoice(reader *bufio.Reader, stdout io.Writer, prompt string, min, max, defaultVal int) int {
	for {
		fmt.Fprint(stdout, prompt)
		line := promptLine(reader)
		if line == "" {
			return defaultVal
		}
		n, err := strconv.Atoi(line)
		if err == nil && n >= min && n <= max {
			return n
		}
		fmt.Fprintf(stdout, "  Please enter %d-%d: ", min, max)
	}
}

func promptLine(reader *bufio.Reader) string {
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func promptYesNo(reader *bufio.Reader, defaultYes bool) bool {
	line := promptLine(reader)
	if line == "" {
		return defaultYes
	}
	lower := strings.ToLower(line)
	return lower == "y" || lower == "yes"
}

func generatePassword() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		b = []byte("fallback-pass-12345678")
	}
	return hex.EncodeToString(b)
}

func boolYesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func detectOSName() string {
	raw, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return runtime.GOOS + " " + runtime.GOARCH
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			name := strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
			return name
		}
	}
	return runtime.GOOS + " " + runtime.GOARCH
}
