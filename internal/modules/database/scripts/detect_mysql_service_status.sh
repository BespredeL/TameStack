for svc in mariadb mysql mysqld; do
    if systemctl is-active --quiet "$svc" 2>/dev/null; then
        echo "active"
        exit 0
    fi
done
if pgrep -x mysqld >/dev/null 2>&1 || pgrep -x mariadbd >/dev/null 2>&1 || pidof mysqld >/dev/null 2>&1 || pidof mariadbd >/dev/null 2>&1; then
    echo "active"
    exit 0
fi
echo "inactive"
