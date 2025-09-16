package updater

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"gopkg.in/yaml.v3"
)

// UpdateAndPushService обновляет тег в файле docker-compose и пушит изменения в репозиторий.
func UpdateAndPushService(repoPath, filePath, serviceName, newTag string) error {
	log.Printf("Starting update for service '%s' in file '%s' to tag '%s'", serviceName, filePath, newTag)

	// 1. Читаем и парсим файл docker-compose.yaml
	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("could not read file: %w", err)
	}

	var data map[string]interface{}
	if err := yaml.Unmarshal(yamlFile, &data); err != nil {
		return fmt.Errorf("could not unmarshal YAML: %w", err)
	}

	// 2. Находим и изменяем тег
	services, ok := data["services"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid services structure in YAML")
	}

	service, ok := services[serviceName].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid service '%s' structure in YAML", serviceName)
	}

	service["image"] = fmt.Sprintf("%s:%s", service["image"].(string)[:strings.LastIndex(service["image"].(string), ":")], newTag)

	// 3. Записываем обновлённый YAML-файл
	updatedYaml, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not marshal YAML: %w", err)
	}

	if err := os.WriteFile(filePath, updatedYaml, 0644); err != nil {
		return fmt.Errorf("could not write updated file: %w", err)
	}
	log.Println("docker-compose.yaml file updated successfully.")

	// 4. Коммитим и пушим изменения
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("could not open git repository at %s: %w", repoPath, err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("could not get worktree: %w", err)
	}

	_, err = worktree.Add(filePath)
	if err != nil {
		return fmt.Errorf("could not add file to git: %w", err)
	}

	commit, err := worktree.Commit(fmt.Sprintf("Update %s to %s", serviceName, newTag), &git.CommitOptions{})
	if err != nil {
		return fmt.Errorf("could not commit changes: %w", err)
	}
	log.Printf("New commit created: %s", commit)

	err = repo.Push(&git.PushOptions{
		// Здесь можно добавить аутентификацию, если она требуется для пуша
	})
	if err != nil {
		return fmt.Errorf("could not push changes: %w", err)
	}
	log.Println("Changes pushed to remote repository successfully.")

	return nil
}
