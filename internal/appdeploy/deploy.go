package appdeploy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/web-casa/llstack/internal/system"
)

// DeployOptions controls application deployment.
type DeployOptions struct {
	AppName    string
	SiteName   string
	Docroot    string
	Backend    string
	PHPVersion string
	DBProvider string
	DBName     string
	DBUser     string
	DBPass     string
	DryRun     bool
}

// DeployResult captures the deployment outcome.
type DeployResult struct {
	App       string `json:"app"`
	Site      string `json:"site"`
	Docroot   string `json:"docroot"`
	DBName    string `json:"db_name,omitempty"`
	DBUser    string `json:"db_user,omitempty"`
	DBPass    string `json:"db_pass,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

// Deploy downloads and configures a PHP application.
func Deploy(ctx context.Context, exec system.Executor, opts DeployOptions) (DeployResult, error) {
	spec, ok := FindApp(opts.AppName)
	if !ok {
		return DeployResult{}, fmt.Errorf("unknown application %q; available: %s", opts.AppName, availableApps())
	}

	result := DeployResult{
		App:     spec.Name,
		Site:    opts.SiteName,
		Docroot: opts.Docroot,
	}

	if opts.DryRun {
		result.Status = "dry-run"
		result.Message = fmt.Sprintf("would deploy %s to %s", spec.DisplayName, opts.Docroot)
		return result, nil
	}

	// Auto-generate DB credentials if needed
	if spec.NeedsDB && opts.DBName == "" {
		safeName := strings.ReplaceAll(strings.ReplaceAll(opts.SiteName, ".", "_"), "-", "_")
		if len(safeName) > 16 {
			safeName = safeName[:16]
		}
		opts.DBName = safeName + "_db"
		opts.DBUser = safeName + "_usr"
		opts.DBPass = generateDeployPassword()
	}

	// Download and extract
	if spec.ExtractMode == "composer" {
		if err := deployComposer(ctx, exec, opts, spec); err != nil {
			return result, err
		}
	} else {
		if err := deployArchive(ctx, exec, opts, spec); err != nil {
			return result, err
		}
	}

	// Post-install steps
	for _, step := range spec.PostInstall {
		if err := runPostInstall(ctx, exec, opts, step); err != nil {
			return result, fmt.Errorf("post-install %s: %w", step.Kind, err)
		}
	}

	// Set permissions
	siteUser := system.SiteUsername(opts.SiteName)
	if siteUser != "" && system.SiteUserExists(siteUser) {
		exec.Run(ctx, system.Command{Name: "chown", Args: []string{"-R", siteUser + ":" + siteUser, opts.Docroot}})
	} else {
		exec.Run(ctx, system.Command{Name: "chown", Args: []string{"-R", "apache:apache", opts.Docroot}})
	}

	// SELinux
	exec.Run(ctx, system.Command{Name: "restorecon", Args: []string{"-Rv", opts.Docroot}})

	result.DBName = opts.DBName
	result.DBUser = opts.DBUser
	result.DBPass = opts.DBPass
	result.Status = "deployed"
	result.Message = fmt.Sprintf("%s deployed to %s", spec.DisplayName, opts.Docroot)

	return result, nil
}

func deployArchive(ctx context.Context, exec system.Executor, opts DeployOptions, spec AppSpec) error {
	tmpDir := filepath.Join(os.TempDir(), "llstack-app-"+spec.Name)
	os.MkdirAll(tmpDir, 0o755)
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, "archive")

	// Download
	dlResult, err := exec.Run(ctx, system.Command{
		Name: "curl", Args: []string{"-fsSL", "-o", archivePath, "-L", spec.DownloadURL},
	})
	if err != nil || dlResult.ExitCode != 0 {
		return fmt.Errorf("download %s failed: %s", spec.DownloadURL, dlResult.Stderr)
	}

	// Extract
	os.MkdirAll(opts.Docroot, 0o755)
	var exRes system.Result
	switch spec.ExtractMode {
	case "tar-strip":
		exRes, err = exec.Run(ctx, system.Command{
			Name: "tar", Args: []string{"xf", archivePath, "--strip-components=1", "-C", opts.Docroot},
		})
	case "zip":
		exRes, err = exec.Run(ctx, system.Command{Name: "unzip", Args: []string{"-o", archivePath, "-d", tmpDir + "/extracted"}})
		if err == nil && exRes.ExitCode == 0 {
			cpRes, cpErr := exec.Run(ctx, system.Command{
				Name: "sh", Args: []string{"-c", fmt.Sprintf("cp -a %s/extracted/*/* %s/ 2>/dev/null || cp -a %s/extracted/* %s/", tmpDir, opts.Docroot, tmpDir, opts.Docroot)},
			})
			if cpErr != nil || cpRes.ExitCode != 0 {
				return fmt.Errorf("copy extracted files failed: %s", cpRes.Stderr)
			}
		}
	default:
		exRes, err = exec.Run(ctx, system.Command{
			Name: "tar", Args: []string{"xf", archivePath, "-C", opts.Docroot},
		})
	}
	if err != nil || exRes.ExitCode != 0 {
		return fmt.Errorf("extract %s failed: %s", spec.Name, exRes.Stderr)
	}

	return nil
}

func deployComposer(ctx context.Context, exec system.Executor, opts DeployOptions, spec AppSpec) error {
	// Clean existing scaffold
	os.RemoveAll(filepath.Join(opts.Docroot, "public"))

	result, err := exec.Run(ctx, system.Command{
		Name: "composer",
		Args: []string{"create-project", "laravel/laravel", opts.Docroot, "--no-interaction", "--prefer-dist"},
	})
	if err != nil || result.ExitCode != 0 {
		return fmt.Errorf("composer create-project failed: %s", result.Stderr)
	}
	return nil
}

func runPostInstall(ctx context.Context, exec system.Executor, opts DeployOptions, step PostInstallStep) error {
	docroot := opts.Docroot
	var res system.Result
	var err error

	switch step.Kind {
	case "copy":
		src := filepath.Join(docroot, step.Source)
		dst := filepath.Join(docroot, step.Target)
		res, err = exec.Run(ctx, system.Command{Name: "cp", Args: []string{src, dst}})
		if err != nil || res.ExitCode != 0 {
			return fmt.Errorf("copy %s -> %s failed: %s", step.Source, step.Target, res.Stderr)
		}

	case "sed":
		target := filepath.Join(docroot, step.Target)
		pattern := step.Pattern
		replace := step.Replace

		replace = strings.ReplaceAll(replace, "{{DB_NAME}}", opts.DBName)
		replace = strings.ReplaceAll(replace, "{{DB_USER}}", opts.DBUser)
		replace = strings.ReplaceAll(replace, "{{DB_PASS}}", opts.DBPass)

		safeReplace := strings.NewReplacer("/", "\\/", "&", "\\&", "\\", "\\\\").Replace(replace)
		safePattern := strings.NewReplacer("/", "\\/", "\\", "\\\\").Replace(pattern)
		res, err = exec.Run(ctx, system.Command{
			Name: "sed", Args: []string{"-i", fmt.Sprintf("s/%s/%s/g", safePattern, safeReplace), target},
		})
		if err != nil || res.ExitCode != 0 {
			return fmt.Errorf("sed %s failed: %s", step.Target, res.Stderr)
		}

	case "chmod":
		target := filepath.Join(docroot, step.Target)
		res, err = exec.Run(ctx, system.Command{Name: "chmod", Args: []string{step.Mode, target}})
		if err != nil || res.ExitCode != 0 {
			return fmt.Errorf("chmod %s %s failed: %s", step.Mode, step.Target, res.Stderr)
		}

	case "command":
		res, err = exec.Run(ctx, system.Command{
			Name: "sh", Args: []string{"-c", fmt.Sprintf("cd '%s' && %s", strings.ReplaceAll(docroot, "'", "'\"'\"'"), step.Command)},
		})
		if err != nil || res.ExitCode != 0 {
			return fmt.Errorf("command failed: %s", res.Stderr)
		}
	}
	return nil
}

func generateDeployPassword() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		b = []byte("fallback-pass-12345678")
	}
	return hex.EncodeToString(b)
}

func availableApps() string {
	names := make([]string, 0)
	for _, app := range Catalog() {
		names = append(names, app.Name)
	}
	return strings.Join(names, ", ")
}
