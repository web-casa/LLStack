package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/web-casa/llstack/internal/config"
	"github.com/web-casa/llstack/internal/core/apply"
	"github.com/web-casa/llstack/internal/core/plan"
	"github.com/web-casa/llstack/internal/logging"
	"github.com/web-casa/llstack/internal/system"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// Manager manages database providers and their managed metadata.
type Manager struct {
	cfg     config.RuntimeConfig
	logger  logging.Logger
	exec    system.Executor
	applier apply.FileApplier
}

// NewManager constructs a database manager.
func NewManager(cfg config.RuntimeConfig, logger logging.Logger, exec system.Executor) Manager {
	return Manager{
		cfg:     cfg,
		logger:  logger,
		exec:    exec,
		applier: apply.NewFileApplier(cfg.Paths.BackupsDir),
	}
}

// Install plans or applies a provider installation.
func (m Manager) Install(ctx context.Context, opts InstallOptions) (plan.Plan, error) {
	spec, err := ResolveProvider(m.cfg, opts.Provider, opts.Version)
	if err != nil {
		return plan.Plan{}, err
	}
	tlsProfile, tlsWarnings := BuildTLSProfile(m.cfg, spec, opts.TLSMode)
	manifestPath := m.manifestPath(spec.Name)

	p := plan.New("db.install", "Install database provider "+string(spec.Name))
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	p.Warnings = append(p.Warnings, spec.Warnings...)
	p.Warnings = append(p.Warnings, tlsWarnings...)
	p.AddOperation(plan.Operation{ID: "install-packages", Kind: "package.install", Target: strings.Join(spec.Packages, " "), Details: map[string]string{"provider": string(spec.Name), "service": spec.ServiceName}})
	p.AddOperation(plan.Operation{ID: "enable-service", Kind: "service.enable", Target: spec.ServiceName})
	if tlsProfile.Enabled {
		p.AddOperation(plan.Operation{ID: "write-tls-config", Kind: "write_file", Target: tlsProfile.ServerConfigPath})
	}
	p.AddOperation(plan.Operation{ID: "write-provider-manifest", Kind: "write_file", Target: manifestPath})

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	if err := m.runDNF(ctx, spec.Packages...); err != nil {
		return p, err
	}
	if err := m.runSystemctl(ctx, "enable", "--now", spec.ServiceName); err != nil {
		return p, err
	}

	if tlsProfile.Enabled {
		if err := os.MkdirAll(filepath.Dir(tlsProfile.ServerConfigPath), 0o755); err != nil {
			return p, err
		}
		if _, err := m.applier.WriteFile(tlsProfile.ServerConfigPath, []byte(RenderTLSConfigSnippet(spec, tlsProfile)), 0o644); err != nil {
			return p, err
		}
		tlsProfile.Status = "configured"
	}

	now := time.Now().UTC()
	manifest := ProviderManifest{
		Provider:     spec.Name,
		Version:      spec.Version,
		Family:       spec.Family,
		ServiceName:  spec.ServiceName,
		Packages:     append([]string{}, spec.Packages...),
		Capabilities: spec.Capabilities,
		TLS:          tlsProfile,
		Status:       "installed",
		Warnings:     append(append([]string{}, spec.Warnings...), tlsWarnings...),
		InstalledAt:  now,
		UpdatedAt:    now,
	}
	if _, err := m.applier.WriteJSON(manifestPath, manifest, 0o644); err != nil {
		return p, err
	}

	m.logger.Info("database provider installed", "provider", spec.Name)
	return p, nil
}

// Init initializes a provider and writes managed connection info.
func (m Manager) Init(ctx context.Context, opts InitOptions) (plan.Plan, error) {
	spec, manifest, err := m.resolveManifest(opts.Provider, opts.DryRun || opts.PlanOnly)
	if err != nil {
		return plan.Plan{}, err
	}
	if opts.AdminUser == "" {
		opts.AdminUser = "llstack_admin"
	}
	if opts.TLSMode == "" {
		opts.TLSMode = manifest.TLS.Mode
	}
	tlsProfile, tlsWarnings := BuildTLSProfile(m.cfg, spec, opts.TLSMode)
	connection := ConnectionInfo{
		Name:         string(spec.Name) + "-admin",
		Host:         "127.0.0.1",
		Port:         spec.Port,
		User:         opts.AdminUser,
		PasswordFile: m.credentialPath(spec.Name, string(spec.Name)+"-admin"),
		TLSMode:      tlsProfile.Mode,
		SSLFlags:     tlsProfile.ClientFlags,
		CreatedAt:    time.Now().UTC(),
	}

	p := plan.New("db.init", "Initialize database provider "+string(spec.Name))
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	p.Warnings = append(p.Warnings, manifest.Warnings...)
	p.Warnings = append(p.Warnings, tlsWarnings...)
	if tlsProfile.Enabled {
		p.AddOperation(plan.Operation{ID: "write-tls-config", Kind: "write_file", Target: tlsProfile.ServerConfigPath})
	}
	p.AddOperation(plan.Operation{ID: "enable-service", Kind: "service.enable", Target: spec.ServiceName})
	for idx, stmt := range initStatements(spec, opts.AdminUser, opts.AdminPassword, opts.TLSMode) {
		p.AddOperation(plan.Operation{ID: fmt.Sprintf("run-init-sql-%d", idx+1), Kind: "sql.exec", Target: spec.ClientBin, Details: map[string]string{"sql": stmt}})
	}
	p.AddOperation(plan.Operation{ID: "write-connection-info", Kind: "write_file", Target: m.connectionPath(spec.Name, connection.Name)})
	if opts.AdminPassword != "" {
		p.AddOperation(plan.Operation{ID: "write-admin-credential", Kind: "write_file", Target: connection.PasswordFile, Details: map[string]string{"scope": "db-credential", "connection": connection.Name}})
	}
	p.AddOperation(plan.Operation{ID: "write-provider-manifest", Kind: "write_file", Target: m.manifestPath(spec.Name)})

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	if spec.Name == ProviderPostgres {
		if err := m.runPostgreSQLSetup(ctx, spec); err != nil {
			return p, err
		}
	}
	if err := m.runSystemctl(ctx, "enable", "--now", spec.ServiceName); err != nil {
		return p, err
	}
	if tlsProfile.Enabled {
		if err := os.MkdirAll(filepath.Dir(tlsProfile.ServerConfigPath), 0o755); err != nil {
			return p, err
		}
		if _, err := m.applier.WriteFile(tlsProfile.ServerConfigPath, []byte(RenderTLSConfigSnippet(spec, tlsProfile)), 0o644); err != nil {
			return p, err
		}
		tlsProfile.Status = "configured"
	}
	for _, stmt := range initStatements(spec, opts.AdminUser, opts.AdminPassword, opts.TLSMode) {
		if err := m.execSQL(ctx, spec, stmt); err != nil {
			return p, err
		}
	}

	now := time.Now().UTC()
	connection.CreatedAt = now
	manifest.TLS = tlsProfile
	manifest.Status = "initialized"
	manifest.Warnings = uniqueStrings(append(manifest.Warnings, tlsWarnings...))
	manifest.AdminConnection = &connection
	manifest.InitializedAt = now
	manifest.UpdatedAt = now
	if opts.AdminPassword != "" {
		if _, err := m.writeCredentialFile(connection.PasswordFile, opts.AdminPassword); err != nil {
			return p, err
		}
	}
	if _, err := m.applier.WriteJSON(m.connectionPath(spec.Name, connection.Name), connection, 0o644); err != nil {
		return p, err
	}
	if _, err := m.applier.WriteJSON(m.manifestPath(spec.Name), manifest, 0o644); err != nil {
		return p, err
	}
	return p, nil
}

// CreateDatabase creates a managed database.
func (m Manager) CreateDatabase(ctx context.Context, opts CreateDatabaseOptions) (plan.Plan, error) {
	spec, manifest, err := m.resolveManifest(opts.Provider, opts.DryRun || opts.PlanOnly)
	if err != nil {
		return plan.Plan{}, err
	}
	if err := validateIdentifier(opts.Name); err != nil {
		return plan.Plan{}, err
	}
	if opts.Owner != "" {
		if err := validateIdentifier(opts.Owner); err != nil {
			return plan.Plan{}, err
		}
	}
	if opts.Encoding == "" {
		opts.Encoding = "UTF8"
	}
	stmt := createDatabaseStatement(spec, opts.Name, opts.Owner, opts.Encoding)

	p := plan.New("db.create", "Create database "+opts.Name+" on "+string(spec.Name))
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	p.AddOperation(plan.Operation{ID: "create-database", Kind: "sql.exec", Target: spec.ClientBin, Details: map[string]string{"sql": stmt}})
	p.AddOperation(plan.Operation{ID: "write-provider-manifest", Kind: "write_file", Target: m.manifestPath(spec.Name)})

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	if err := m.execSQL(ctx, spec, stmt); err != nil {
		return p, err
	}
	now := time.Now().UTC()
	manifest.Databases = appendOrReplaceDatabase(manifest.Databases, DatabaseRecord{Name: opts.Name, Owner: opts.Owner, Encoding: opts.Encoding, CreatedAt: now})
	manifest.UpdatedAt = now
	if _, err := m.applier.WriteJSON(m.manifestPath(spec.Name), manifest, 0o644); err != nil {
		return p, err
	}
	return p, nil
}

// CreateUser creates a managed database user and grants privileges.
func (m Manager) CreateUser(ctx context.Context, opts CreateUserOptions) (plan.Plan, error) {
	spec, manifest, err := m.resolveManifest(opts.Provider, opts.DryRun || opts.PlanOnly)
	if err != nil {
		return plan.Plan{}, err
	}
	if err := validateIdentifier(opts.Name); err != nil {
		return plan.Plan{}, err
	}
	if opts.Database != "" {
		if err := validateIdentifier(opts.Database); err != nil {
			return plan.Plan{}, err
		}
	}
	if opts.Password == "" {
		return plan.Plan{}, fmt.Errorf("password is required")
	}
	if opts.Privileges == "" {
		opts.Privileges = "ALL PRIVILEGES"
	}
	if opts.TLSMode == "" {
		opts.TLSMode = manifest.TLS.Mode
	}
	statements := createUserStatements(spec, opts)

	p := plan.New("db.user.create", "Create database user "+opts.Name+" on "+string(spec.Name))
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly
	for idx, stmt := range statements {
		p.AddOperation(plan.Operation{ID: fmt.Sprintf("sql-%d", idx+1), Kind: "sql.exec", Target: spec.ClientBin, Details: map[string]string{"sql": stmt}})
	}
	p.AddOperation(plan.Operation{ID: "write-provider-manifest", Kind: "write_file", Target: m.manifestPath(spec.Name)})
	if opts.Database != "" {
		connection := ConnectionInfo{
			Name:         opts.Name + "@" + opts.Database,
			Host:         "127.0.0.1",
			Port:         spec.Port,
			User:         opts.Name,
			Database:     opts.Database,
			PasswordFile: m.credentialPath(spec.Name, opts.Name+"@"+opts.Database),
			TLSMode:      opts.TLSMode,
			SSLFlags:     manifest.TLS.ClientFlags,
			CreatedAt:    time.Now().UTC(),
		}
		p.AddOperation(plan.Operation{ID: "write-connection-info", Kind: "write_file", Target: m.connectionPath(spec.Name, connection.Name)})
		p.AddOperation(plan.Operation{ID: "write-user-credential", Kind: "write_file", Target: connection.PasswordFile, Details: map[string]string{"scope": "db-credential", "connection": connection.Name}})
	}

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	for _, stmt := range statements {
		if err := m.execSQL(ctx, spec, stmt); err != nil {
			return p, err
		}
	}
	now := time.Now().UTC()
	manifest.Users = appendOrReplaceUser(manifest.Users, UserRecord{
		Name:       opts.Name,
		Database:   opts.Database,
		Privileges: opts.Privileges,
		TLSMode:    opts.TLSMode,
		CreatedAt:  now,
	})
	manifest.UpdatedAt = now
	if opts.Database != "" {
		connection := ConnectionInfo{
			Name:         opts.Name + "@" + opts.Database,
			Host:         "127.0.0.1",
			Port:         spec.Port,
			User:         opts.Name,
			Database:     opts.Database,
			PasswordFile: m.credentialPath(spec.Name, opts.Name+"@"+opts.Database),
			TLSMode:      opts.TLSMode,
			SSLFlags:     manifest.TLS.ClientFlags,
			CreatedAt:    now,
		}
		if _, err := m.writeCredentialFile(connection.PasswordFile, opts.Password); err != nil {
			return p, err
		}
		if _, err := m.applier.WriteJSON(m.connectionPath(spec.Name, connection.Name), connection, 0o644); err != nil {
			return p, err
		}
	}
	if _, err := m.applier.WriteJSON(m.manifestPath(spec.Name), manifest, 0o644); err != nil {
		return p, err
	}
	return p, nil
}

// List returns all managed provider manifests.
func (m Manager) List() ([]ProviderManifest, error) {
	dir := m.cfg.DB.ManagedProvidersDir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]ProviderManifest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var manifest ProviderManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, err
		}
		out = append(out, manifest)
	}
	slices.SortFunc(out, func(a, b ProviderManifest) int {
		return strings.Compare(string(a.Provider), string(b.Provider))
	})
	return out, nil
}

// Reconcile rewrites managed provider metadata and TLS config from the stored manifest.
func (m Manager) Reconcile(ctx context.Context, opts ReconcileOptions) (plan.Plan, error) {
	spec, manifest, err := m.resolveManifest(opts.Provider, false)
	if err != nil {
		return plan.Plan{}, err
	}

	p := plan.New("db.reconcile", "Reconcile database provider "+string(spec.Name))
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly

	adminConnPath := ""
	if manifest.AdminConnection != nil {
		adminConnPath = m.connectionPath(spec.Name, manifest.AdminConnection.Name)
		if !pathExists(adminConnPath) {
			p.AddOperation(plan.Operation{ID: "write-admin-connection", Kind: "write_file", Target: adminConnPath, Details: map[string]string{"scope": "db-connection", "provider": string(spec.Name)}})
		}
	}
	if manifest.TLS.Enabled && !pathExists(manifest.TLS.ServerConfigPath) {
		p.AddOperation(plan.Operation{ID: "write-tls-config", Kind: "write_file", Target: manifest.TLS.ServerConfigPath, Details: map[string]string{"scope": "db-tls", "provider": string(spec.Name)}})
	}
	p.AddOperation(plan.Operation{ID: "write-provider-manifest", Kind: "write_file", Target: m.manifestPath(spec.Name)})

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	if manifest.TLS.Enabled && !pathExists(manifest.TLS.ServerConfigPath) {
		if err := os.MkdirAll(filepath.Dir(manifest.TLS.ServerConfigPath), 0o755); err != nil {
			return p, err
		}
		if _, err := m.applier.WriteFile(manifest.TLS.ServerConfigPath, []byte(RenderTLSConfigSnippet(spec, manifest.TLS)), 0o644); err != nil {
			return p, err
		}
	}
	if manifest.AdminConnection != nil && !pathExists(adminConnPath) {
		if _, err := m.applier.WriteJSON(adminConnPath, manifest.AdminConnection, 0o644); err != nil {
			return p, err
		}
	}
	manifest.UpdatedAt = time.Now().UTC()
	if _, err := m.applier.WriteJSON(m.manifestPath(spec.Name), manifest, 0o644); err != nil {
		return p, err
	}
	return p, nil
}

func (m Manager) resolveManifest(provider ProviderName, allowMissing bool) (ProviderSpec, ProviderManifest, error) {
	if provider == "" {
		manifests, err := m.List()
		if err != nil {
			return ProviderSpec{}, ProviderManifest{}, err
		}
		if len(manifests) != 1 {
			return ProviderSpec{}, ProviderManifest{}, fmt.Errorf("provider is required when multiple or zero database providers are managed")
		}
		provider = manifests[0].Provider
	}
	spec, err := ResolveProvider(m.cfg, provider, "")
	if err != nil {
		return ProviderSpec{}, ProviderManifest{}, err
	}
	raw, err := os.ReadFile(m.manifestPath(provider))
	if err != nil {
		if allowMissing && os.IsNotExist(err) {
			return spec, ProviderManifest{
				Provider:     spec.Name,
				Version:      spec.Version,
				Family:       spec.Family,
				ServiceName:  spec.ServiceName,
				Capabilities: spec.Capabilities,
				TLS: DatabaseTLSProfile{
					Mode: TLSDisabled,
				},
				Status: "planned",
			}, nil
		}
		return ProviderSpec{}, ProviderManifest{}, err
	}
	var manifest ProviderManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return ProviderSpec{}, ProviderManifest{}, err
	}
	return spec, manifest, nil
}

func (m Manager) manifestPath(provider ProviderName) string {
	return filepath.Join(m.cfg.DB.ManagedProvidersDir, string(provider)+".json")
}

func (m Manager) connectionPath(provider ProviderName, name string) string {
	safeName := strings.NewReplacer("@", "_", "/", "_", " ", "_").Replace(name)
	return filepath.Join(m.cfg.DB.ManagedConnectionsDir, string(provider)+"-"+safeName+".json")
}

func (m Manager) credentialsDir() string {
	return filepath.Join(m.cfg.Paths.ConfigDir, "db", "credentials")
}

func (m Manager) credentialPath(provider ProviderName, name string) string {
	safeName := strings.NewReplacer("@", "_", "/", "_", " ", "_").Replace(name)
	return filepath.Join(m.credentialsDir(), string(provider)+"-"+safeName+".secret")
}

func (m Manager) writeCredentialFile(path string, password string) (apply.Change, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return apply.Change{}, err
	}
	content := []byte(password)
	if !strings.HasSuffix(password, "\n") {
		content = append(content, '\n')
	}
	return m.applier.WriteFile(path, content, 0o600)
}

func (m Manager) runDNF(ctx context.Context, packages ...string) error {
	if len(packages) == 0 {
		return nil
	}
	result, err := m.exec.Run(ctx, system.Command{Name: "dnf", Args: append([]string{"install", "-y"}, packages...)})
	if err != nil {
		return fmt.Errorf("dnf install: %w (%s)", err, result.Stderr)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("dnf exited with %d: %s", result.ExitCode, result.Stderr)
	}
	return nil
}

func (m Manager) runSystemctl(ctx context.Context, args ...string) error {
	result, err := m.exec.Run(ctx, system.Command{Name: "systemctl", Args: args})
	if err != nil {
		return fmt.Errorf("systemctl %s: %w (%s)", strings.Join(args, " "), err, result.Stderr)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("systemctl %s exited with %d: %s", strings.Join(args, " "), result.ExitCode, result.Stderr)
	}
	return nil
}

func (m Manager) runPostgreSQLSetup(ctx context.Context, spec ProviderSpec) error {
	cmd := "postgresql-" + spec.Version + "-setup"
	result, err := m.exec.Run(ctx, system.Command{Name: cmd, Args: []string{"initdb"}})
	if err != nil {
		return fmt.Errorf("%s initdb: %w (%s)", cmd, err, result.Stderr)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("%s initdb exited with %d: %s", cmd, result.ExitCode, result.Stderr)
	}
	return nil
}

func (m Manager) execSQL(ctx context.Context, spec ProviderSpec, statement string) error {
	var cmd system.Command
	switch spec.Family {
	case "mysql":
		cmd = system.Command{Name: spec.ClientBin, Args: []string{"-uroot", "-e", statement}}
	case "postgresql":
		cmd = system.Command{Name: "runuser", Args: []string{"-u", "postgres", "--", "psql", "-v", "ON_ERROR_STOP=1", "-d", "postgres", "-c", statement}}
	default:
		return fmt.Errorf("unsupported database family %q", spec.Family)
	}
	result, err := m.exec.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("sql exec: %w (%s)", err, result.Stderr)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("sql exec exited with %d: %s", result.ExitCode, result.Stderr)
	}
	return nil
}

func initStatements(spec ProviderSpec, user, password string, mode TLSMode) []string {
	if password == "" {
		return nil
	}
	switch spec.Family {
	case "mysql":
		sslClause := ""
		if mode == TLSRequired {
			sslClause = " REQUIRE SSL"
		}
		return []string{
			fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'%s;", user, escapeSQLString(password), sslClause),
			fmt.Sprintf("GRANT ALL PRIVILEGES ON *.* TO '%s'@'localhost' WITH GRANT OPTION;", user),
			"FLUSH PRIVILEGES;",
		}
	case "postgresql":
		return []string{
			fmt.Sprintf("DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s') THEN CREATE ROLE \"%s\" LOGIN PASSWORD '%s'; ELSE ALTER ROLE \"%s\" LOGIN PASSWORD '%s'; END IF; END $$;", user, user, escapeSQLString(password), user, escapeSQLString(password)),
			fmt.Sprintf("ALTER ROLE \"%s\" CREATEDB CREATEROLE;", user),
		}
	default:
		return nil
	}
}

func createDatabaseStatement(spec ProviderSpec, name, owner, encoding string) string {
	switch spec.Family {
	case "mysql":
		return fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", name)
	case "postgresql":
		if owner != "" {
			return fmt.Sprintf("CREATE DATABASE \"%s\" WITH OWNER \"%s\" ENCODING '%s';", name, owner, encoding)
		}
		return fmt.Sprintf("CREATE DATABASE \"%s\" WITH ENCODING '%s';", name, encoding)
	default:
		return ""
	}
}

func createUserStatements(spec ProviderSpec, opts CreateUserOptions) []string {
	switch spec.Family {
	case "mysql":
		sslClause := ""
		if opts.TLSMode == TLSRequired {
			sslClause = " REQUIRE SSL"
		}
		statements := []string{
			fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'%s;", opts.Name, escapeSQLString(opts.Password), sslClause),
		}
		if opts.Database != "" {
			statements = append(statements, fmt.Sprintf("GRANT %s ON `%s`.* TO '%s'@'localhost';", opts.Privileges, opts.Database, opts.Name))
			statements = append(statements, "FLUSH PRIVILEGES;")
		}
		return statements
	case "postgresql":
		statements := []string{
			fmt.Sprintf("DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s') THEN CREATE ROLE \"%s\" LOGIN PASSWORD '%s'; ELSE ALTER ROLE \"%s\" LOGIN PASSWORD '%s'; END IF; END $$;", opts.Name, opts.Name, escapeSQLString(opts.Password), opts.Name, escapeSQLString(opts.Password)),
		}
		if opts.Database != "" {
			statements = append(statements, fmt.Sprintf("GRANT CONNECT ON DATABASE \"%s\" TO \"%s\";", opts.Database, opts.Name))
		}
		return statements
	default:
		return nil
	}
}

func validateIdentifier(value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("identifier %q must match %s", value, identifierPattern.String())
	}
	return nil
}

func escapeSQLString(value string) string {
	return strings.ReplaceAll(value, `'`, `''`)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}

func appendOrReplaceDatabase(values []DatabaseRecord, next DatabaseRecord) []DatabaseRecord {
	for idx := range values {
		if values[idx].Name == next.Name {
			values[idx] = next
			return values
		}
	}
	return append(values, next)
}

func appendOrReplaceUser(values []UserRecord, next UserRecord) []UserRecord {
	for idx := range values {
		if values[idx].Name == next.Name && values[idx].Database == next.Database {
			values[idx] = next
			return values
		}
	}
	return append(values, next)
}

// UninstallOptions controls DB provider removal.
type UninstallOptions struct {
	Provider ProviderName
	DryRun   bool
	PlanOnly bool
}

// Uninstall plans or applies removal of a managed DB provider.
func (m Manager) Uninstall(ctx context.Context, opts UninstallOptions) (plan.Plan, error) {
	p := plan.New("db.uninstall", fmt.Sprintf("Remove database provider %s", opts.Provider))
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly

	manifests, err := m.List()
	if err != nil {
		return plan.Plan{}, err
	}
	var manifest *ProviderManifest
	for _, mf := range manifests {
		if mf.Provider == opts.Provider {
			manifest = &mf
			break
		}
	}
	if manifest == nil {
		return plan.Plan{}, fmt.Errorf("no managed manifest found for provider %s", opts.Provider)
	}

	serviceName := manifest.ServiceName
	if serviceName != "" {
		p.AddOperation(plan.Operation{ID: "stop-db-" + string(opts.Provider), Kind: "service.stop", Target: serviceName})
		p.AddOperation(plan.Operation{ID: "disable-db-" + string(opts.Provider), Kind: "service.disable", Target: serviceName})
	}

	spec, _ := ResolveProvider(m.cfg, opts.Provider, manifest.Version)
	if spec.Name != "" {
		for _, pkg := range spec.Packages {
			p.AddOperation(plan.Operation{ID: "remove-pkg-" + pkg, Kind: "package.remove", Target: pkg})
		}
	}

	manifestPath := filepath.Join(m.cfg.DB.ManagedProvidersDir, string(opts.Provider)+".json")
	p.AddOperation(plan.Operation{ID: "remove-db-manifest", Kind: "file.delete", Target: manifestPath})

	p.Warnings = append(p.Warnings, "database data directories are preserved; remove manually if needed")

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	if serviceName != "" {
		m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"stop", serviceName}})
		m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"disable", serviceName}})
	}
	if spec.Name != "" && len(spec.Packages) > 0 {
		args := append([]string{"-y", "remove"}, spec.Packages...)
		if _, err := m.exec.Run(ctx, system.Command{Name: "dnf", Args: args}); err != nil {
			return p, fmt.Errorf("dnf remove failed: %w", err)
		}
	}
	os.Remove(manifestPath)

	m.logger.Info("database provider uninstalled", "provider", string(opts.Provider))
	return p, nil
}

// BackupOptions controls database backup.
type BackupOptions struct {
	Provider  ProviderName
	OutputDir string
	DryRun    bool
	PlanOnly  bool
}

// Backup plans or executes a database backup using provider-native tools.
func (m Manager) Backup(ctx context.Context, opts BackupOptions) (plan.Plan, error) {
	p := plan.New("db.backup", fmt.Sprintf("Backup database provider %s", opts.Provider))
	p.DryRun = opts.DryRun
	p.PlanOnly = opts.PlanOnly

	manifests, err := m.List()
	if err != nil {
		return plan.Plan{}, err
	}
	var manifest *ProviderManifest
	for _, mf := range manifests {
		if mf.Provider == opts.Provider {
			manifest = &mf
			break
		}
	}
	if manifest == nil {
		return plan.Plan{}, fmt.Errorf("no managed manifest found for provider %s", opts.Provider)
	}

	spec, err := ResolveProvider(m.cfg, opts.Provider, manifest.Version)
	if err != nil {
		return plan.Plan{}, err
	}

	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(m.cfg.Paths.BackupsDir, "db", string(opts.Provider))
	}
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	dumpFile := filepath.Join(outputDir, fmt.Sprintf("%s-%s.sql", opts.Provider, timestamp))

	cmd, ok := buildBackupCommand(spec, manifest, dumpFile)
	if !ok {
		return plan.Plan{}, fmt.Errorf("backup not supported for provider family %s", spec.Family)
	}

	p.AddOperation(plan.Operation{
		ID:     "backup-" + string(opts.Provider),
		Kind:   "db.backup",
		Target: dumpFile,
		Details: map[string]string{
			"provider": string(opts.Provider),
			"family":   spec.Family,
		},
	})

	if opts.DryRun || opts.PlanOnly {
		return p, nil
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return p, err
	}

	result, err := m.exec.Run(ctx, cmd)
	if err != nil {
		return p, fmt.Errorf("backup command failed: %w", err)
	}
	if result.ExitCode != 0 {
		return p, fmt.Errorf("backup command exited with %d: %s", result.ExitCode, result.Stderr)
	}

	m.logger.Info("database backup completed", "provider", string(opts.Provider), "output", dumpFile)
	return p, nil
}

// ScheduleBackupOptions controls scheduled backup configuration.
type ScheduleBackupOptions struct {
	Provider  ProviderName
	OutputDir string
	Retain    int // number of backups to keep (0 = unlimited)
}

// BackupTimerUnit generates a systemd timer for scheduled backups.
func BackupTimerUnit(provider ProviderName) string {
	return fmt.Sprintf(`[Unit]
Description=LLStack %s Backup Timer

[Timer]
OnCalendar=daily
Persistent=true
RandomizedDelaySec=1800

[Install]
WantedBy=timers.target
`, provider)
}

// BackupServiceUnit generates the systemd service for scheduled backups.
func BackupServiceUnit(provider ProviderName, llstackBin string) string {
	return fmt.Sprintf(`[Unit]
Description=LLStack %s Backup

[Service]
Type=oneshot
ExecStart=%s db:backup %s
`, provider, llstackBin, provider)
}

// EnableScheduledBackup installs systemd timer for daily backups.
func (m Manager) EnableScheduledBackup(ctx context.Context, opts ScheduleBackupOptions) error {
	llstackBin := "/usr/local/bin/llstack"

	timerPath := fmt.Sprintf("/etc/systemd/system/llstack-backup-%s.timer", opts.Provider)
	servicePath := fmt.Sprintf("/etc/systemd/system/llstack-backup-%s.service", opts.Provider)

	if err := os.MkdirAll(filepath.Dir(timerPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(timerPath, []byte(BackupTimerUnit(opts.Provider)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(servicePath, []byte(BackupServiceUnit(opts.Provider, llstackBin)), 0o644); err != nil {
		return err
	}

	m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"daemon-reload"}})
	m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"enable", "--now", fmt.Sprintf("llstack-backup-%s.timer", opts.Provider)}})

	m.logger.Info("scheduled backup enabled", "provider", string(opts.Provider))
	return nil
}

// DisableScheduledBackup removes the scheduled backup timer.
func (m Manager) DisableScheduledBackup(ctx context.Context, provider ProviderName) error {
	timerName := fmt.Sprintf("llstack-backup-%s.timer", provider)
	m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"disable", "--now", timerName}})
	os.Remove(fmt.Sprintf("/etc/systemd/system/%s", timerName))
	os.Remove(fmt.Sprintf("/etc/systemd/system/llstack-backup-%s.service", provider))
	m.exec.Run(ctx, system.Command{Name: "systemctl", Args: []string{"daemon-reload"}})
	return nil
}

// CleanupOldBackups removes old backups keeping only the most recent N files.
func CleanupOldBackups(dir string, retain int) (int, error) {
	if retain <= 0 {
		return 0, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	// Filter .sql files and sort by name (timestamped, newest last)
	var sqlFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			sqlFiles = append(sqlFiles, filepath.Join(dir, e.Name()))
		}
	}
	if len(sqlFiles) <= retain {
		return 0, nil
	}
	sort.Strings(sqlFiles)
	toDelete := sqlFiles[:len(sqlFiles)-retain]
	for _, f := range toDelete {
		os.Remove(f)
	}
	return len(toDelete), nil
}

func buildBackupCommand(spec ProviderSpec, manifest *ProviderManifest, outputFile string) (system.Command, bool) {
	switch spec.Family {
	case "mysql":
		args := []string{"--all-databases", "--single-transaction"}
		if manifest.AdminConnection != nil {
			if host := strings.TrimSpace(manifest.AdminConnection.Host); host != "" {
				args = append(args, "--host", host)
			}
			if manifest.AdminConnection.Port > 0 {
				args = append(args, "--port", fmt.Sprintf("%d", manifest.AdminConnection.Port))
			}
			if user := strings.TrimSpace(manifest.AdminConnection.User); user != "" {
				args = append(args, "--user", user)
			}
		}
		args = append(args, "--result-file="+outputFile)
		return system.Command{Name: "mysqldump", Args: args}, true
	case "postgresql":
		args := []string{"--file=" + outputFile}
		if manifest.AdminConnection != nil {
			if host := strings.TrimSpace(manifest.AdminConnection.Host); host != "" {
				args = append(args, "--host", host)
			}
			if manifest.AdminConnection.Port > 0 {
				args = append(args, "--port", fmt.Sprintf("%d", manifest.AdminConnection.Port))
			}
			if user := strings.TrimSpace(manifest.AdminConnection.User); user != "" {
				args = append(args, "--username", user)
			}
		}
		return system.Command{Name: "pg_dumpall", Args: args}, true
	default:
		return system.Command{}, false
	}
}

func pathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
