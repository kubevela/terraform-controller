package gitrepo

import (
	"github.com/go-git/go-git/v5"
)

// Clone is equal to `git clone`
func Clone(url, path string) error {
	_, err := git.PlainClone(path, false, &git.CloneOptions{URL: url})
	return err
}
