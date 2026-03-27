package site

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/web-casa/llstack/internal/core/model"
	"github.com/web-casa/llstack/internal/core/render"
)

// ScaffoldAsset renders profile-specific starter files under the document root.
func ScaffoldAssets(site model.Site) []render.Asset {
	switch site.Profile {
	case ProfileWordPress:
		return []render.Asset{
			{
				Path:    filepath.Join(site.DocumentRoot, "index.php"),
				Content: []byte("<?php\n// LLStack placeholder for WordPress deployment.\nrequire __DIR__ . '/wp-blog-header.php';\n"),
				Mode:    0o644,
			},
			{
				Path:    filepath.Join(site.DocumentRoot, ".htaccess"),
				Content: []byte(wordPressHtaccess()),
				Mode:    0o644,
			},
			{
				Path:    filepath.Join(site.DocumentRoot, "wp-config-sample.php"),
				Content: []byte(wordPressConfigSample(site)),
				Mode:    0o644,
			},
			{
				Path:    filepath.Join(site.DocumentRoot, "wp-content", "uploads", ".gitkeep"),
				Content: []byte{},
				Mode:    0o644,
			},
			{
				Path:    filepath.Join(site.DocumentRoot, "README-LLSTACK.md"),
				Content: []byte(wordPressReadme(site)),
				Mode:    0o644,
			},
		}
	case ProfileLaravel:
		projectRoot := site.DocumentRoot
		if filepath.Base(site.DocumentRoot) == "public" {
			projectRoot = filepath.Dir(site.DocumentRoot)
		}
		return []render.Asset{
			{
				Path:    filepath.Join(site.DocumentRoot, "index.php"),
				Content: []byte("<?php\n// LLStack placeholder for Laravel public entrypoint.\nrequire __DIR__ . '/../vendor/autoload.php';\n$app = require_once __DIR__ . '/../bootstrap/app.php';\n$app->run();\n"),
				Mode:    0o644,
			},
			{
				Path:    filepath.Join(projectRoot, ".env.example"),
				Content: []byte(laravelEnvExample(site)),
				Mode:    0o644,
			},
			{
				Path:    filepath.Join(projectRoot, "artisan"),
				Content: []byte(laravelArtisanStub()),
				Mode:    0o755,
			},
			{
				Path:    filepath.Join(projectRoot, "bootstrap", "app.php"),
				Content: []byte(laravelBootstrapStub()),
				Mode:    0o644,
			},
			{
				Path:    filepath.Join(projectRoot, "routes", "web.php"),
				Content: []byte(laravelRoutesStub()),
				Mode:    0o644,
			},
			{
				Path:    filepath.Join(projectRoot, "storage", "app", ".gitkeep"),
				Content: []byte{},
				Mode:    0o644,
			},
			{
				Path:    filepath.Join(projectRoot, "storage", "logs", ".gitkeep"),
				Content: []byte{},
				Mode:    0o644,
			},
			{
				Path:    filepath.Join(projectRoot, "composer.json"),
				Content: []byte(laravelComposerStub()),
				Mode:    0o644,
			},
			{
				Path:    filepath.Join(projectRoot, "README-LLSTACK.md"),
				Content: []byte(laravelReadme(site)),
				Mode:    0o644,
			},
		}
	case ProfileStatic:
		return []render.Asset{
			{
				Path:    filepath.Join(site.DocumentRoot, "index.html"),
				Content: []byte(staticIndex(site)),
				Mode:    0o644,
			},
		}
	case ProfileReverseProxy:
		return []render.Asset{
			{
				Path:    filepath.Join(site.DocumentRoot, "index.html"),
				Content: []byte(reverseProxyIndex(site)),
				Mode:    0o644,
			},
		}
	default:
		phpBody := "<?php\nheader('Content-Type: text/plain');\necho \"LLStack site ready: " + site.Domain.ServerName + "\\n\";\n"
		if site.Profile != "" {
			phpBody += "echo \"profile: " + site.Profile + "\\n\";\n"
		}
		phpBody += "?>\n"
		return []render.Asset{
			{
				Path:    filepath.Join(site.DocumentRoot, "index.php"),
				Content: []byte(phpBody),
				Mode:    0o644,
			},
		}
	}
}

func wordPressHtaccess() string {
	return strings.Join([]string{
		"# BEGIN WordPress",
		"<IfModule mod_rewrite.c>",
		"RewriteEngine On",
		"RewriteBase /",
		"RewriteRule ^index\\.php$ - [L]",
		"RewriteCond %{REQUEST_FILENAME} !-f",
		"RewriteCond %{REQUEST_FILENAME} !-d",
		"RewriteRule . /index.php [L]",
		"</IfModule>",
		"# END WordPress",
		"",
	}, "\n")
}

func wordPressConfigSample(site model.Site) string {
	return strings.Join([]string{
		"<?php",
		"define('DB_NAME', 'wordpress');",
		"define('DB_USER', 'wp_user');",
		"define('DB_PASSWORD', 'change-me');",
		"define('DB_HOST', '127.0.0.1');",
		"define('DB_CHARSET', 'utf8mb4');",
		"define('DB_COLLATE', '');",
		"$table_prefix = 'wp_';",
		"define('WP_HOME', 'https://" + site.Domain.ServerName + "');",
		"define('WP_SITEURL', 'https://" + site.Domain.ServerName + "');",
		"if (!defined('ABSPATH')) {",
		"    define('ABSPATH', __DIR__ . '/');",
		"}",
		"require_once ABSPATH . 'wp-settings.php';",
		"",
	}, "\n")
}

func wordPressReadme(site model.Site) string {
	return strings.Join([]string{
		"# LLStack WordPress Scaffold",
		"",
		"站点: " + site.Domain.ServerName,
		"",
		"该目录只提供 WordPress 的受管入口骨架，不会自动下载 WordPress 核心。",
		"建议后续步骤：",
		"1. 下载并解压 WordPress 到当前 document root",
		"2. 按需调整 `wp-config-sample.php`",
		"3. 通过 `llstack site:ssl` 与数据库向导补齐 TLS 和 DB",
		"",
	}, "\n")
}

func laravelEnvExample(site model.Site) string {
	return strings.Join([]string{
		"APP_NAME=LLStack",
		"APP_ENV=production",
		"APP_KEY=",
		"APP_DEBUG=false",
		"APP_URL=https://" + site.Domain.ServerName,
		"LOG_CHANNEL=stack",
		"DB_CONNECTION=mysql",
		"DB_HOST=127.0.0.1",
		"DB_PORT=3306",
		"DB_DATABASE=app",
		"DB_USERNAME=app",
		"DB_PASSWORD=change-me",
		"",
	}, "\n")
}

func laravelArtisanStub() string {
	return strings.Join([]string{
		"#!/usr/bin/env php",
		"<?php",
		"// LLStack placeholder artisan entrypoint.",
		"echo \"Laravel scaffold generated by LLStack. Install the full app before using artisan.\\n\";",
		"",
	}, "\n")
}

func laravelBootstrapStub() string {
	return strings.Join([]string{
		"<?php",
		"// LLStack placeholder bootstrap/app.php for Laravel-style layout.",
		"return new class {",
		"    public function run(): void",
		"    {",
		"        echo 'LLStack Laravel scaffold';",
		"    }",
		"};",
		"",
	}, "\n")
}

func laravelRoutesStub() string {
	return strings.Join([]string{
		"<?php",
		"// LLStack placeholder routes/web.php",
		"return [",
		"    ['GET', '/', 'welcome'],",
		"];",
		"",
	}, "\n")
}

func laravelComposerStub() string {
	return strings.Join([]string{
		"{",
		`  "name": "llstack/laravel-scaffold",`,
		`  "description": "LLStack placeholder scaffold for a Laravel deployment.",`,
		`  "type": "project",`,
		`  "require": {},`,
		`  "scripts": {`,
		`    "post-root-package-install": [`,
		`      "@php -r \"echo 'Install the real Laravel app before running composer workflows.';\""`,
		`    ]`,
		`  }`,
		"}",
		"",
	}, "\n")
}

func laravelReadme(site model.Site) string {
	return strings.Join([]string{
		"# LLStack Laravel Scaffold",
		"",
		"站点: " + site.Domain.ServerName,
		"public root: " + site.DocumentRoot,
		"",
		"该目录是 Laravel 风格目录结构占位，不会自动执行 `composer create-project`。",
		"建议后续步骤：",
		"1. 在项目根目录安装真实 Laravel 应用",
		"2. 生成 `APP_KEY` 并完善 `.env`",
		"3. 通过 `llstack db:*` 和 `llstack site:ssl` 补齐 DB/TLS",
		"",
	}, "\n")
}

func staticIndex(site model.Site) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
</head>
<body>
  <main>
    <h1>%s</h1>
    <p>Static site scaffolded by LLStack.</p>
  </main>
</body>
</html>
`, site.Domain.ServerName, site.Domain.ServerName)
}

func reverseProxyIndex(site model.Site) string {
	upstream := "-"
	if len(site.ReverseProxyRules) > 0 {
		upstream = site.ReverseProxyRules[0].Upstream
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>%s proxy</title>
</head>
<body>
  <main>
    <h1>%s</h1>
    <p>Reverse proxy site scaffolded by LLStack.</p>
    <p>Upstream: %s</p>
  </main>
</body>
</html>
`, site.Domain.ServerName, site.Domain.ServerName, upstream)
}
