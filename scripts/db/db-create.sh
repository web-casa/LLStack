#!/bin/bash
set -euo pipefail

# Create a database with user and grant privileges
# Usage: db-create.sh --engine <mariadb|mysql|postgresql> --name <db_name> --db-user <user> --password-file <path>

ENGINE=""
DB_NAME=""
DB_USER=""
PW_FILE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --engine)        ENGINE="$2"; shift 2 ;;
        --name)          DB_NAME="$2"; shift 2 ;;
        --db-user)       DB_USER="$2"; shift 2 ;;
        --password-file) PW_FILE="$2"; shift 2 ;;
        *) echo '{"ok": false, "error": "unknown_arg"}' >&2; exit 1 ;;
    esac
done

if [[ -z "$ENGINE" || -z "$DB_NAME" ]]; then
    echo '{"ok": false, "error": "missing_args", "message": "--engine and --name required"}' >&2
    exit 1
fi

# Validate database name (alphanumeric + underscore)
if ! echo "$DB_NAME" | grep -qP '^[a-zA-Z][a-zA-Z0-9_]{0,63}$'; then
    echo '{"ok": false, "error": "invalid_db_name"}' >&2
    exit 1
fi

# Read password from file
DB_PASS=""
if [[ -n "$PW_FILE" && -f "$PW_FILE" ]]; then
    DB_PASS=$(cat "$PW_FILE")
    rm -f "$PW_FILE"
fi

case "$ENGINE" in
    mariadb|mysql)
        # Validate DB user format
        if [[ -n "$DB_USER" ]] && ! echo "$DB_USER" | grep -qP '^[a-zA-Z][a-zA-Z0-9_]{0,31}$'; then
            echo '{"ok": false, "error": "invalid_db_user"}' >&2; exit 1
        fi

        # Create database
        mysql -e "CREATE DATABASE IF NOT EXISTS \`$DB_NAME\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

        # Create user and grant (password via stdin to avoid shell escaping issues)
        if [[ -n "$DB_USER" && -n "$DB_PASS" ]]; then
            # Use mysql_native_password explicitly (MariaDB 10.11+ defaults to unix_socket)
            ESCAPED_PASS=$(printf '%s' "$DB_PASS" | sed "s/'/''/g")
            mysql -e "CREATE USER IF NOT EXISTS '$DB_USER'@'localhost' IDENTIFIED VIA mysql_native_password USING PASSWORD('$ESCAPED_PASS');" 2>/dev/null || \
            mysql -e "CREATE USER IF NOT EXISTS '$DB_USER'@'localhost' IDENTIFIED BY '$ESCAPED_PASS';"
            mysql -e "GRANT ALL PRIVILEGES ON \`$DB_NAME\`.* TO '$DB_USER'@'localhost';"
            mysql -e "FLUSH PRIVILEGES;"
        fi

        HOST="localhost"
        PORT=3306
        ;;

    postgresql)
        # Create user (escape single quotes in password)
        if [[ -n "$DB_USER" && -n "$DB_PASS" ]]; then
            ESCAPED_PASS="${DB_PASS//\'/\'\'}"
            sudo -u postgres psql -c "CREATE USER \"$DB_USER\" WITH PASSWORD '${ESCAPED_PASS}';" 2>/dev/null || true
        fi

        # Create database
        sudo -u postgres psql -c "CREATE DATABASE \"$DB_NAME\" OWNER \"${DB_USER:-postgres}\";" 2>/dev/null

        HOST="localhost"
        PORT=5432
        ;;

    *)
        echo '{"ok": false, "error": "unsupported_engine"}' >&2
        exit 1
        ;;
esac

cat << EOF
{"ok": true, "data": {"name": "$DB_NAME", "engine": "$ENGINE", "user": "$DB_USER", "host": "$HOST", "port": $PORT}}
EOF
