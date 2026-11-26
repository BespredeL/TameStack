package apps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tamestack/internal/db"
	"tamestack/internal/ssh"
)

// AllowedArtisanCommands defines the strict whitelist of permitted artisan operations.
var AllowedArtisanCommands = map[string]struct {
	IsDangerous bool
	Description string
}{
	"migrate":        {IsDangerous: true, Description: "Выполнение миграций базы данных"},
	"migrate:fresh":  {IsDangerous: true, Description: "Полный пересоздание таблиц БД и выполнение миграций заново"},
	"migrate:status": {IsDangerous: false, Description: "Просмотр статуса миграций"},
	"db:seed":        {IsDangerous: true, Description: "Наполнение базы данных тестовыми сидами"},
	"cache:clear":    {IsDangerous: false, Description: "Очистка кэша приложения"},
	"config:cache":   {IsDangerous: false, Description: "Кэширование файлов конфигурации"},
	"config:clear":   {IsDangerous: false, Description: "Сброс кэша конфигурации"},
	"route:cache":    {IsDangerous: false, Description: "Кэширование таблицы маршрутов"},
	"route:clear":    {IsDangerous: false, Description: "Сброс кэша маршрутов"},
	"view:cache":     {IsDangerous: false, Description: "Компиляция представлений Blade"},
	"view:clear":     {IsDangerous: false, Description: "Очистка скомпилированных Blade шаблонов"},
	"storage:link":   {IsDangerous: false, Description: "Создание символической ссылки public/storage -> storage/app/public"},
	"optimize":       {IsDangerous: false, Description: "Оптимизация конфигурации и маршрутов"},
	"optimize:clear": {IsDangerous: false, Description: "Очистка всех оптимизированных кэшей"},
	"key:generate":   {IsDangerous: true, Description: "Генерация нового ключа приложения APP_KEY"},
}

// IsAllowedArtisanCommand checks whether the command is in the whitelist.
func IsAllowedArtisanCommand(cmd string) (bool, bool) {
	cmd = strings.TrimSpace(strings.ToLower(cmd))
	spec, ok := AllowedArtisanCommands[cmd]
	if !ok {
		return false, false
	}
	return true, spec.IsDangerous
}

// RunArtisanCommand validates and securely executes an artisan command from the whitelist.
func RunArtisanCommand(ctx context.Context, client *ssh.SSHClient, site *db.SiteRecord, cmd string, confirmDangerous bool) (*ArtisanCommandResult, error) {
	cleanCmd := strings.TrimSpace(strings.ToLower(cmd))

	// Strict shell safety check.
	if strings.ContainsAny(cleanCmd, ";|&`$><\n\r") {
		return nil, fmt.Errorf("command contains forbidden characters")
	}

	allowed, isDangerous := IsAllowedArtisanCommand(cleanCmd)
	if !allowed {
		return nil, fmt.Errorf("команда 'artisan %s' отклонена: не входит в разрешенный белый список команд управления", cleanCmd)
	}

	if isDangerous && !confirmDangerous {
		return nil, fmt.Errorf("команда 'artisan %s' требует явного подтверждения оператора перед выполнением", cleanCmd)
	}

	projectRoot := ResolveProjectRoot(site.DocumentRoot)
	artisanFile := fmt.Sprintf("%s/artisan", projectRoot)
	if !client.FileExists(ctx, artisanFile) {
		return nil, fmt.Errorf("artisan не найден в каталоге проекта: %s", projectRoot)
	}

	startTime := time.Now()

	// Execute as www-data or default web user.
	user := "www-data"
	var flags string
	if cleanCmd == "migrate" {
		flags = "--force --no-interaction"
	} else if cleanCmd == "key:generate" {
		flags = "--force"
	}

	innerCmd := fmt.Sprintf("php artisan %s", cleanCmd)
	if flags != "" {
		innerCmd += " " + flags
	}
	innerCmd += " 2>&1"
	shellCmd := fmt.Sprintf("cd %s && su -s /bin/bash %s -c %s",
		ssh.ShellEscape(projectRoot),
		ssh.ShellEscape(user),
		ssh.ShellEscape(innerCmd),
	)

	out, stderr, code, err := client.NewCommand(shellCmd).WithSudo().Exec(ctx)
	duration := time.Since(startTime).Milliseconds()

	combined := strings.TrimSpace(string(out) + "\n" + string(stderr))

	success := (err == nil && code == 0)
	return &ArtisanCommandResult{
		Command:    cleanCmd,
		Output:     combined,
		Success:    success,
		DurationMs: duration,
	}, nil
}
