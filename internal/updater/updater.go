package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// UpdateAndPushService updates the image tag of a specified service in a docker-compose.yaml file,
// commits the change to a Git repository, and pushes the commit to the remote repository.
func UpdateAndPushService(repoPath, filePath, serviceName, newTag, SSHKeyPath, authorName, authorEmail string) error {
	logrus.Infof("Starting update for service '%s' in file '%s' to tag '%s'", serviceName, filePath, newTag)

	// Reading and updating the docker-compose.yaml file
	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("could not read file: %w", err)
	}

	var data map[string]interface{}
	if err := yaml.Unmarshal(yamlFile, &data); err != nil {
		return fmt.Errorf("could not unmarshal YAML: %w", err)
	}

	// Finding and updating the service image tag
	services, ok := data["services"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid services structure in YAML")
	}

	service, ok := services[serviceName].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid service '%s' structure in YAML", serviceName)
	}

	service["image"] = fmt.Sprintf("%s:%s", service["image"].(string)[:strings.LastIndex(service["image"].(string), ":")], newTag)

	// 3. Writing the updated YAML back to the file
	updatedYaml, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not marshal YAML: %w", err)
	}

	if err := os.WriteFile(filePath, updatedYaml, 0644); err != nil {
		return fmt.Errorf("could not write updated file: %w", err)
	}
	logrus.Infof("docker-compose.yaml file updated successfully.")

	// 4. Committing and pushing the changes to the Git repository
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open Git repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	relativePath, err := filepath.Rel(repoPath, filePath)
	if err != nil {
		return fmt.Errorf("could not get relative path: %w", err)
	}

	_, err = worktree.Add(relativePath)
	if err != nil {
		return fmt.Errorf("could not add file to git: %w", err)
	}

	commit, err := worktree.Commit(fmt.Sprintf("Update %s to %s", serviceName, newTag), &git.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("could not commit changes: %w", err)
	}
	logrus.Infof("New commit created: %s", commit)

	publicKeys, err := ssh.NewPublicKeysFromFile("git", SSHKeyPath, "")
	if err != nil {
		return fmt.Errorf("failed to read SSH key: %w", err)
	}
	auth := publicKeys
	err = repo.Push(&git.PushOptions{
		Auth: auth,
	})
	if err != nil {
		return fmt.Errorf("could not push changes: %w", err)
	}
	logrus.Infof("Changes pushed to remote repository successfully.")

	return nil
}
