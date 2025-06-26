// Package container contains helpers for assembling containers used in Terraform jobs.
package container

import (
	"github.com/oam-dev/terraform-controller/api/types"
	v1 "k8s.io/api/core/v1"
)

// Assembler helps to assemble the init containers
type Assembler struct {
	Name string

	GitCredential              bool
	TerraformCredential        bool
	TerraformRC                bool
	TerraformCredentialsHelper bool

	TerraformImage string
	BusyboxImage   string
	GitImage       string

	Git  types.Git
	Envs []v1.EnvVar
}

// NewAssembler creates a new Assembler with the given name.
func NewAssembler(name string) *Assembler {
	return &Assembler{Name: name}
}

// GitCredReference sets the Git credential secret.
func (a *Assembler) GitCredReference(ptr *v1.SecretReference) *Assembler {
	if ptr != nil {
		a.GitCredential = true
	}
	return a
}

// TerraformCredReference sets the Terraform credential secret.
func (a *Assembler) TerraformCredReference(ptr *v1.SecretReference) *Assembler {
	if ptr != nil {
		a.TerraformCredential = true
	}
	return a
}

// TerraformRCReference sets the Terraform rc file secret.
func (a *Assembler) TerraformRCReference(ptr *v1.SecretReference) *Assembler {
	if ptr != nil {
		a.TerraformRC = true
	}
	return a
}

// TerraformCredentialsHelperReference sets the Terraform credentials helper secret.
func (a *Assembler) TerraformCredentialsHelperReference(ptr *v1.SecretReference) *Assembler {
	if ptr != nil {
		a.TerraformCredentialsHelper = true
	}
	return a
}

// SetTerraformImage specifies the Terraform image to use.
func (a *Assembler) SetTerraformImage(image string) *Assembler {
	a.TerraformImage = image
	return a
}

// SetBusyboxImage specifies the BusyBox image to use.
func (a *Assembler) SetBusyboxImage(image string) *Assembler {
	a.BusyboxImage = image
	return a
}

// SetGitImage specifies the Git image to use.
func (a *Assembler) SetGitImage(image string) *Assembler {
	a.GitImage = image
	return a
}

// SetGit sets the Git configuration.
func (a *Assembler) SetGit(git types.Git) *Assembler {
	a.Git = git
	return a
}

// SetEnvs sets additional environment variables for containers.
func (a *Assembler) SetEnvs(envs []v1.EnvVar) *Assembler {
	a.Envs = envs
	return a
}
