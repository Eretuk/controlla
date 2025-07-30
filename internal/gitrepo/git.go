package gitrepo

import (
	"fmt"
	"os"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

func CloneOrPull(repoURL, branch, dstPath, sshKeyPath string) error {
	var auth transport.AuthMethod

	publicKeys, err := ssh.NewPublicKeysFromFile("git", sshKeyPath, "")
	if err != nil {
		return fmt.Errorf("failed to read SSH key: %w", err)
	}
	auth = publicKeys

	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		// Clone
		_, err := git.PlainClone(dstPath, false, &git.CloneOptions{
			URL:           repoURL,
			Progress:      os.Stdout,
			Auth:          auth,
			ReferenceName: plumbing.NewBranchReferenceName(branch),
			SingleBranch:  true,
			Depth:         1,
		})
		if err != nil {
			return fmt.Errorf("failed to clone: %w", err)
		}
	} else {
		// Pull
		repo, err := git.PlainOpen(dstPath)
		if err != nil {
			return fmt.Errorf("failed to open repo: %w", err)
		}

		w, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf("failed to get worktree: %w", err)
		}

		err = w.Pull(&git.PullOptions{
			Auth:          auth,
			RemoteName:    "origin",
			ReferenceName: plumbing.NewBranchReferenceName(branch),
			SingleBranch:  true,
		})

		if err != nil && err != git.NoErrAlreadyUpToDate {
			return fmt.Errorf("failed to pull: %w", err)
		}
	}

	return nil
}
