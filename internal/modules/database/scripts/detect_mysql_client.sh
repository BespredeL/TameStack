export PATH=$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
DEFAULTS={{.Defaults}}

test_cmd() {
    eval "$1 -N -e 'SELECT 1;' >/dev/null 2>&1"
}

# 1. Debian/Ubuntu /etc/mysql/debian.cnf (Standard package configuration on Ubuntu 20.04/Debian)
if [ -f /etc/mysql/debian.cnf ]; then
    if test_cmd "mysql --defaults-file=/etc/mysql/debian.cnf"; then
        echo "mysql --defaults-file=/etc/mysql/debian.cnf"
        exit 0
    elif test_cmd "sudo -n mysql --defaults-file=/etc/mysql/debian.cnf"; then
        echo "sudo -n mysql --defaults-file=/etc/mysql/debian.cnf"
        exit 0
    elif test_cmd "sudo mysql --defaults-file=/etc/mysql/debian.cnf"; then
        echo "sudo mysql --defaults-file=/etc/mysql/debian.cnf"
        exit 0
    fi
fi

# 2. Check /root/.my.cnf
if [ -f /root/.my.cnf ]; then
    if test_cmd "mysql --defaults-extra-file=/root/.my.cnf"; then
        echo "mysql --defaults-extra-file=/root/.my.cnf"
        exit 0
    elif test_cmd "sudo -n mysql --defaults-extra-file=/root/.my.cnf"; then
        echo "sudo -n mysql --defaults-extra-file=/root/.my.cnf"
        exit 0
    elif test_cmd "sudo mysql --defaults-extra-file=/root/.my.cnf"; then
        echo "sudo mysql --defaults-extra-file=/root/.my.cnf"
        exit 0
    fi
fi

# 3. Direct local client (root with auth_socket or standard user)
if test_cmd "mysql"; then
    echo "mysql"
    exit 0
elif test_cmd "sudo -n -H mysql"; then
    echo "sudo -n -H mysql"
    exit 0
elif test_cmd "sudo -H mysql"; then
    echo "sudo -H mysql"
    exit 0
elif test_cmd "sudo -n mysql"; then
    echo "sudo -n mysql"
    exit 0
elif test_cmd "sudo mysql"; then
    echo "sudo mysql"
    exit 0
fi

# 4. MariaDB equivalents
if [ -f /etc/mysql/debian.cnf ]; then
    if test_cmd "mariadb --defaults-file=/etc/mysql/debian.cnf"; then
        echo "mariadb --defaults-file=/etc/mysql/debian.cnf"
        exit 0
    elif test_cmd "sudo -n mariadb --defaults-file=/etc/mysql/debian.cnf"; then
        echo "sudo -n mariadb --defaults-file=/etc/mysql/debian.cnf"
        exit 0
    elif test_cmd "sudo mariadb --defaults-file=/etc/mysql/debian.cnf"; then
        echo "sudo mariadb --defaults-file=/etc/mysql/debian.cnf"
        exit 0
    fi
fi

if test_cmd "mariadb"; then
    echo "mariadb"
    exit 0
elif test_cmd "sudo -n -H mariadb"; then
    echo "sudo -n -H mariadb"
    exit 0
elif test_cmd "sudo -H mariadb"; then
    echo "sudo -H mariadb"
    exit 0
elif test_cmd "sudo -n mariadb"; then
    echo "sudo -n mariadb"
    exit 0
elif test_cmd "sudo mariadb"; then
    echo "sudo mariadb"
    exit 0
fi

# 5. Extract debian-sys-maint credentials from debian.cnf if readable via sudo
if [ -f /etc/mysql/debian.cnf ]; then
    U=$(sudo -n grep -m 1 -E '^\s*user\s*=' /etc/mysql/debian.cnf 2>/dev/null | awk '{print $3}' || sudo grep -m 1 -E '^\s*user\s*=' /etc/mysql/debian.cnf 2>/dev/null | awk '{print $3}')
    P=$(sudo -n grep -m 1 -E '^\s*password\s*=' /etc/mysql/debian.cnf 2>/dev/null | awk '{print $3}' || sudo grep -m 1 -E '^\s*password\s*=' /etc/mysql/debian.cnf 2>/dev/null | awk '{print $3}')
    S=$(sudo -n grep -m 1 -E '^\s*socket\s*=' /etc/mysql/debian.cnf 2>/dev/null | awk '{print $3}' || sudo grep -m 1 -E '^\s*socket\s*=' /etc/mysql/debian.cnf 2>/dev/null | awk '{print $3}')
    if [ -n "$U" ] && [ -n "$P" ]; then
        umask 077
        f=$(mktemp /tmp/.sh_my.XXXXXX) || true
        if [ -n "$f" ]; then
            printf '[client]\nuser=%s\npassword=%s\n' "$U" "$P" > "$f"
            if [ -n "$S" ]; then
                printf 'socket=%s\n' "$S" >> "$f"
            fi
            chmod 600 "$f"
            C="mysql --defaults-extra-file=$f"
            if test_cmd "$C"; then
                echo "$C"
                exit 0
            fi
        fi
    fi
fi

# 6. Defaults file written by TameStack (password is not on the command line)
if [ -n "$DEFAULTS" ]; then
    if run_sudo mysql --defaults-extra-file="$DEFAULTS" -N -e "SELECT 1;" >/dev/null 2>&1; then
        echo "run_sudo mysql --defaults-extra-file=$DEFAULTS"
        exit 0
    elif mysql --defaults-extra-file="$DEFAULTS" -N -e "SELECT 1;" >/dev/null 2>&1; then
        echo "mysql --defaults-extra-file=$DEFAULTS"
        exit 0
    fi
fi

# 7. Fallback to safest default
if command -v mysql >/dev/null 2>&1; then
    if [ -f /etc/mysql/debian.cnf ]; then
        echo "sudo -n mysql --defaults-file=/etc/mysql/debian.cnf"
    else
        echo "mysql"
    fi
else
    echo "mariadb"
fi
