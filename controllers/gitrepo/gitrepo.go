package gitrepo

import (
	"github.com/go-git/go-git/v5"
)

func Clone(url, path string) error {
	_, err := git.PlainClone(path, false,&git.CloneOptions{URL: url})
	return err
}
