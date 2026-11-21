#!/usr/bin/env bash
# TameStack Standalone Log Discovery Script
# Discovers and outputs web server access/error logs safely under sudo elevation.
# Parameters are passed via environment variables: DOMAIN, LOGTYPE, LINES, DOCROOT, IS_APACHE, IS_NGINX

[ -z "$DOMAIN" ] && exit 0
[ -z "$LOGTYPE" ] && LOGTYPE="access"
[ -z "$LINES" ] && LINES=100

CLEAN_DOM=$(echo "$DOMAIN" | sed 's/^www\.//')
WWW_DOM="www.$CLEAN_DOM"

# Safe execution helper: run_sudo or sudo or direct
do_sudo() {
    if command -v run_sudo >/dev/null 2>&1; then
        run_sudo "$@"
    elif [ "$(id -u)" -eq 0 ]; then
        "$@"
    elif command -v sudo >/dev/null 2>&1; then
        sudo -n "$@" 2>/dev/null || sudo "$@"
    else
        "$@"
    fi
}

# 1. Candidate paths based on web server type (covering dots, underscores, dashes, domains subdirs)
CANDIDATES=""
if [ "$IS_APACHE" = "true" ]; then
    CANDIDATES="
/var/log/apache2/${DOMAIN}.${LOGTYPE}.log
/var/log/apache2/${CLEAN_DOM}.${LOGTYPE}.log
/var/log/apache2/${WWW_DOM}.${LOGTYPE}.log
/var/log/apache2/${DOMAIN}_${LOGTYPE}.log
/var/log/apache2/${CLEAN_DOM}_${LOGTYPE}.log
/var/log/apache2/${WWW_DOM}_${LOGTYPE}.log
/var/log/apache2/${DOMAIN}-${LOGTYPE}.log
/var/log/apache2/${CLEAN_DOM}-${LOGTYPE}.log
/var/log/apache2/${DOMAIN}.${LOGTYPE}
/var/log/apache2/${CLEAN_DOM}.${LOGTYPE}
/var/log/apache2/${DOMAIN}.log
/var/log/apache2/${CLEAN_DOM}.log
/var/log/apache2/${DOMAIN}/${LOGTYPE}.log
/var/log/apache2/${CLEAN_DOM}/${LOGTYPE}.log
/var/log/apache2/domains/${DOMAIN}.${LOGTYPE}.log
/var/log/apache2/domains/${CLEAN_DOM}.${LOGTYPE}.log
/var/log/apache2/domains/${DOMAIN}.log
/var/log/apache2/domains/${CLEAN_DOM}.log
/var/log/httpd/${DOMAIN}.${LOGTYPE}.log
/var/log/httpd/${CLEAN_DOM}.${LOGTYPE}.log
/var/log/httpd/${DOMAIN}_${LOGTYPE}.log
/var/log/httpd/${CLEAN_DOM}_${LOGTYPE}.log
/var/log/httpd/${DOMAIN}-${LOGTYPE}.log
/var/log/httpd/${DOMAIN}_${LOGTYPE}_log
/var/log/httpd/${CLEAN_DOM}_${LOGTYPE}_log
"
fi

if [ "$IS_NGINX" = "true" ]; then
    CANDIDATES="$CANDIDATES
/var/log/nginx/${DOMAIN}.${LOGTYPE}.log
/var/log/nginx/${CLEAN_DOM}.${LOGTYPE}.log
/var/log/nginx/${WWW_DOM}.${LOGTYPE}.log
/var/log/nginx/${DOMAIN}_${LOGTYPE}.log
/var/log/nginx/${CLEAN_DOM}_${LOGTYPE}.log
/var/log/nginx/${DOMAIN}-${LOGTYPE}.log
/var/log/nginx/${CLEAN_DOM}-${LOGTYPE}.log
/var/log/nginx/${DOMAIN}.${LOGTYPE}
/var/log/nginx/${DOMAIN}.log
/var/log/nginx/${DOMAIN}/${LOGTYPE}.log
/var/log/nginx/domains/${DOMAIN}.log
"
fi

# Test candidate files
for cand in $CANDIDATES; do
    if [ -n "$cand" ] && do_sudo test -f "$cand"; then
        echo "=== [Файл логов: $cand] ==="
        OUT=$(do_sudo tail -n "$LINES" "$cand" 2>/dev/null)
        if [ -n "$OUT" ]; then
            echo "$OUT"
        else
            echo "(Файл журнала пуст)"
        fi
        exit 0
    fi
done

# 2. Dynamic discovery in web server log directories
SEARCH_LOG_DIRS=""
if [ "$IS_APACHE" = "true" ]; then
    SEARCH_LOG_DIRS="/var/log/apache2 /var/log/httpd"
fi
if [ "$IS_NGINX" = "true" ]; then
    SEARCH_LOG_DIRS="$SEARCH_LOG_DIRS /var/log/nginx"
fi

if [ -n "$SEARCH_LOG_DIRS" ]; then
    DYN_LOG=$(do_sudo find $SEARCH_LOG_DIRS /var/log/virtualmin /var/log/ispconfig -maxdepth 3 -type f \( -name "*${CLEAN_DOM}*${LOGTYPE}*" -o -name "*${DOMAIN}*${LOGTYPE}*" \) 2>/dev/null | head -n 1)
    if [ -n "$DYN_LOG" ] && do_sudo test -f "$DYN_LOG"; then
        echo "=== [Файл логов: $DYN_LOG] ==="
        OUT=$(do_sudo tail -n "$LINES" "$DYN_LOG" 2>/dev/null)
        if [ -n "$OUT" ]; then
            echo "$OUT"
        else
            echo "(Файл журнала пуст)"
        fi
        exit 0
    fi
fi

# 3. DocumentRoot or /home/ logs
if [ -n "$DOCROOT" ] && [ -d "$DOCROOT" ]; then
    DOC_LOGS=$(do_sudo find "$DOCROOT/.." -maxdepth 3 -type f \( -name "*${LOGTYPE}*.log" -o -name "*${CLEAN_DOM}*${LOGTYPE}*" \) 2>/dev/null | head -n 1)
    if [ -n "$DOC_LOGS" ] && do_sudo test -f "$DOC_LOGS"; then
        echo "=== [Файл логов: $DOC_LOGS] ==="
        OUT=$(do_sudo tail -n "$LINES" "$DOC_LOGS" 2>/dev/null)
        if [ -n "$OUT" ]; then
            echo "$OUT"
        else
            echo "(Файл журнала пуст)"
        fi
        exit 0
    fi
fi

HOME_LOGS=$(do_sudo find /home -maxdepth 4 -type f \( -name "*${CLEAN_DOM}*${LOGTYPE}*" -o -name "*${DOMAIN}*${LOGTYPE}*" \) 2>/dev/null | head -n 1)
if [ -n "$HOME_LOGS" ] && do_sudo test -f "$HOME_LOGS"; then
    echo "=== [Файл логов: $HOME_LOGS] ==="
    OUT=$(do_sudo tail -n "$LINES" "$HOME_LOGS" 2>/dev/null)
    if [ -n "$OUT" ]; then
        echo "$OUT"
    else
        echo "(Файл журнала пуст)"
    fi
    exit 0
fi

# 4. For Apache on Debian/Ubuntu: check other_vhosts_access.log
if [ "$IS_APACHE" = "true" ] && [ "$LOGTYPE" = "access" ] && do_sudo test -f /var/log/apache2/other_vhosts_access.log; then
    MATCHES=$(do_sudo grep -i "$CLEAN_DOM" /var/log/apache2/other_vhosts_access.log 2>/dev/null | tail -n "$LINES")
    if [ -n "$MATCHES" ]; then
        echo "=== [Лог-файл: /var/log/apache2/other_vhosts_access.log (записи домена $DOMAIN)] ==="
        echo "$MATCHES"
        exit 0
    fi
fi

# 5. Filter general web server logs for domain
GEN_LOGS=""
if [ "$IS_APACHE" = "true" ]; then
    GEN_LOGS="/var/log/apache2/${LOGTYPE}.log /var/log/httpd/${LOGTYPE}.log /var/log/httpd/${LOGTYPE}_log"
fi
if [ "$IS_NGINX" = "true" ]; then
    GEN_LOGS="$GEN_LOGS /var/log/nginx/${LOGTYPE}.log"
fi

for gen in $GEN_LOGS; do
    if [ -n "$gen" ] && do_sudo test -f "$gen"; then
        MATCHES=$(do_sudo grep -i "$CLEAN_DOM" "$gen" 2>/dev/null | tail -n "$LINES")
        if [ -n "$MATCHES" ]; then
            echo "=== [Лог-файл: $gen (записи с упоминанием $DOMAIN)] ==="
            echo "$MATCHES"
            exit 0
        fi
    fi
done

# 6. Fallback: output recent entries of general log if available
for gen in $GEN_LOGS; do
    if [ -n "$gen" ] && do_sudo test -f "$gen"; then
        echo "=== [Общий лог сервера: $gen (индивидуальный лог для $DOMAIN не найден)] ==="
        do_sudo tail -n "$LINES" "$gen" 2>/dev/null || echo "(Файл журнала пуст)"
        exit 0
    fi
done

echo "Лог-файлы для домена $DOMAIN не найдены в стандартных каталогах веб-сервера."
