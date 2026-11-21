package policies

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"tamestack/internal/ai/domain"
)

// ExecutionPlan encapsulates all information needed for a human to review and approve an action.
type ExecutionPlan struct {
	ID                 string           `json:"id"`
	Title              string           `json:"title"`
	ServerID           string           `json:"server_id"`
	ActionType         string           `json:"action_type"`
	RiskLevel          domain.RiskLevel `json:"risk_level"`
	TargetFile         string           `json:"target_file,omitempty"`
	Diff               string           `json:"diff,omitempty"`
	ProposedContent    string           `json:"proposed_content,omitempty"`
	BackupPath         string           `json:"backup_path,omitempty"`
	SyntaxCheckCommand string           `json:"syntax_check_command,omitempty"`
	Steps              []string         `json:"steps"`
	CreatedAt          time.Time        `json:"created_at"`
}

// ComputeUnifiedDiff computes a simple, readable line-by-line unified diff between old and new text.
func ComputeUnifiedDiff(fileName, oldContent, newContent string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- a/%s\n", fileName))
	sb.WriteString(fmt.Sprintf("+++ b/%s\n", fileName))

	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		if oldLines[i] == newLines[j] {
			sb.WriteString(fmt.Sprintf(" %s\n", oldLines[i]))
			i++
			j++
		} else {
			matchOld := -1
			for nextI := i + 1; nextI < len(oldLines) && nextI < i+5; nextI++ {
				if oldLines[nextI] == newLines[j] {
					matchOld = nextI
					break
				}
			}

			if matchOld != -1 {
				for ; i < matchOld; i++ {
					sb.WriteString(fmt.Sprintf("-%s\n", oldLines[i]))
				}
			} else {
				sb.WriteString(fmt.Sprintf("-%s\n", oldLines[i]))
				sb.WriteString(fmt.Sprintf("+%s\n", newLines[j]))
				i++
				j++
			}
		}
	}

	for ; i < len(oldLines); i++ {
		sb.WriteString(fmt.Sprintf("-%s\n", oldLines[i]))
	}
	for ; j < len(newLines); j++ {
		sb.WriteString(fmt.Sprintf("+%s\n", newLines[j]))
	}

	return sb.String()
}

// GenerateConfigChangePlan creates an approval plan for modifying a configuration file.
func GenerateConfigChangePlan(serverID, filePath, currentContent, newContent, webserverType string) (*ExecutionPlan, error) {
	gate := NewSecurityGate()
	if err := gate.ValidatePath(filePath); err != nil {
		return nil, err
	}

	diff := ComputeUnifiedDiff(filePath, currentContent, newContent)
	backupPath := fmt.Sprintf("%s.bak_%s", filePath, time.Now().Format("20060102_150405"))

	syntaxCmd := ""
	if strings.Contains(filePath, "nginx") || webserverType == "nginx" {
		syntaxCmd = "nginx -t"
	} else if strings.Contains(filePath, "apache") || strings.Contains(filePath, "httpd") || webserverType == "apache" {
		syntaxCmd = "apache2ctl -t || httpd -t"
	}

	steps := []string{
		fmt.Sprintf("1. Создать резервную копию исходного файла в %s", backupPath),
		fmt.Sprintf("2. Записать обновленный конфигурационный файл в %s", filePath),
	}
	if syntaxCmd != "" {
		steps = append(steps, fmt.Sprintf("3. Проверить синтаксис конфигурации командой '%s'", syntaxCmd))
	}
	steps = append(steps, "4. Применить изменения и перезагрузить службу (reload)")

	return &ExecutionPlan{
		ID:                 "plan_" + uuid.New().String(),
		Title:              fmt.Sprintf("Изменение конфигурации %s", filePath),
		ServerID:           serverID,
		ActionType:         "apply_config_diff",
		RiskLevel:          domain.RiskHigh,
		TargetFile:         filePath,
		Diff:               diff,
		ProposedContent:    newContent,
		BackupPath:         backupPath,
		SyntaxCheckCommand: syntaxCmd,
		Steps:              steps,
		CreatedAt:          time.Now(),
	}, nil
}
