package appdeploy

// AppSpec defines how to deploy a PHP application.
type AppSpec struct {
	Name         string            `json:"name"`
	DisplayName  string            `json:"display_name"`
	Description  string            `json:"description"`
	DownloadURL  string            `json:"download_url"`
	ExtractMode  string            `json:"extract_mode"` // tar-strip, zip, git
	SiteProfile  string            `json:"site_profile"`
	PHPExtensions []string         `json:"php_extensions"`
	MinPHP       string            `json:"min_php"`
	NeedsDB      bool              `json:"needs_db"`
	DBType       string            `json:"db_type"` // mysql, postgresql, any
	PostInstall  []PostInstallStep `json:"post_install"`
}

// PostInstallStep describes an action after file extraction.
type PostInstallStep struct {
	Kind    string `json:"kind"`    // copy, sed, chmod, command
	Source  string `json:"source"`
	Target  string `json:"target"`
	Pattern string `json:"pattern"`
	Replace string `json:"replace"`
	Command string `json:"command"`
	Mode    string `json:"mode"`
}

// Catalog returns all supported applications.
func Catalog() []AppSpec {
	return []AppSpec{
		{
			Name:        "wordpress",
			DisplayName: "WordPress",
			Description: "The most popular CMS",
			DownloadURL: "https://wordpress.org/latest.tar.gz",
			ExtractMode: "tar-strip",
			SiteProfile: "wordpress",
			PHPExtensions: []string{"mysqlnd", "gd", "xml", "mbstring", "zip", "curl", "imagick"},
			MinPHP:      "7.4",
			NeedsDB:     true,
			DBType:      "mysql",
			PostInstall: []PostInstallStep{
				{Kind: "copy", Source: "wp-config-sample.php", Target: "wp-config.php"},
				{Kind: "sed", Target: "wp-config.php", Pattern: "database_name_here", Replace: "{{DB_NAME}}"},
				{Kind: "sed", Target: "wp-config.php", Pattern: "username_here", Replace: "{{DB_USER}}"},
				{Kind: "sed", Target: "wp-config.php", Pattern: "password_here", Replace: "{{DB_PASS}}"},
			},
		},
		{
			Name:        "drupal",
			DisplayName: "Drupal",
			Description: "Enterprise CMS framework",
			DownloadURL: "https://www.drupal.org/download-latest/tar.gz",
			ExtractMode: "tar-strip",
			SiteProfile: "generic",
			PHPExtensions: []string{"gd", "xml", "mbstring", "pdo", "opcache", "curl"},
			MinPHP:      "8.1",
			NeedsDB:     true,
			DBType:      "any",
		},
		{
			Name:        "joomla",
			DisplayName: "Joomla",
			Description: "Flexible CMS platform",
			DownloadURL: "https://downloads.joomla.org/api/v1/latest/cms",
			ExtractMode: "zip",
			SiteProfile: "generic",
			PHPExtensions: []string{"mysqlnd", "gd", "xml", "mbstring", "zip", "curl", "json"},
			MinPHP:      "8.1",
			NeedsDB:     true,
			DBType:      "mysql",
		},
		{
			Name:        "nextcloud",
			DisplayName: "Nextcloud",
			Description: "Self-hosted file sync and collaboration",
			DownloadURL: "https://download.nextcloud.com/server/releases/latest.tar.bz2",
			ExtractMode: "tar-strip",
			SiteProfile: "generic",
			PHPExtensions: []string{"gd", "xml", "mbstring", "zip", "curl", "intl", "bcmath", "gmp", "pdo"},
			MinPHP:      "8.0",
			NeedsDB:     true,
			DBType:      "any",
		},
		{
			Name:        "matomo",
			DisplayName: "Matomo",
			Description: "Open-source web analytics",
			DownloadURL: "https://builds.matomo.org/matomo-latest.tar.gz",
			ExtractMode: "tar-strip",
			SiteProfile: "generic",
			PHPExtensions: []string{"mysqlnd", "gd", "xml", "mbstring", "curl", "json"},
			MinPHP:      "7.4",
			NeedsDB:     true,
			DBType:      "mysql",
		},
		{
			Name:        "mediawiki",
			DisplayName: "MediaWiki",
			Description: "Wiki platform (powers Wikipedia)",
			DownloadURL: "https://releases.wikimedia.org/mediawiki/1.42/mediawiki-1.42.3.tar.gz",
			ExtractMode: "tar-strip",
			SiteProfile: "generic",
			PHPExtensions: []string{"mysqlnd", "xml", "mbstring", "intl", "json"},
			MinPHP:      "8.1",
			NeedsDB:     true,
			DBType:      "any",
		},
		{
			Name:        "typecho",
			DisplayName: "Typecho",
			Description: "Lightweight PHP blogging platform",
			DownloadURL: "https://github.com/typecho/typecho/releases/latest/download/typecho.tar.gz",
			ExtractMode: "tar-strip",
			SiteProfile: "generic",
			PHPExtensions: []string{"mysqlnd", "mbstring", "curl"},
			MinPHP:      "7.4",
			NeedsDB:     true,
			DBType:      "mysql",
		},
		{
			Name:        "laravel",
			DisplayName: "Laravel (new project)",
			Description: "PHP framework via Composer",
			ExtractMode: "composer",
			SiteProfile: "laravel",
			PHPExtensions: []string{"pgsql", "mbstring", "xml", "curl", "zip", "bcmath", "tokenizer"},
			MinPHP:      "8.2",
			NeedsDB:     true,
			DBType:      "any",
		},
	}
}

// FindApp returns the app spec by name.
func FindApp(name string) (AppSpec, bool) {
	for _, app := range Catalog() {
		if app.Name == name {
			return app, true
		}
	}
	return AppSpec{}, false
}
