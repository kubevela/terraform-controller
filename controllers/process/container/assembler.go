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

func NewAssembler(name string) *Assembler {
	return &Assembler{Name: name}
}

func (a *Assembler) GitCredReference(ptr *v1.SecretReference) *Assembler {
	if ptr != nil {
		a.GitCredential = true
	}
	return a
}

func (a *Assembler) TerraformCredReference(ptr *v1.SecretReference) *Assembler {
	if ptr != nil {
		a.TerraformCredential = true
	}
	return a
}

func (a *Assembler) TerraformRCReference(ptr *v1.SecretReference) *Assembler {
	if ptr != nil {
		a.TerraformRC = true
	}
	return a
}

func (a *Assembler) TerraformCredentialsHelperReference(ptr *v1.SecretReference) *Assembler {
	if ptr != nil {
		a.TerraformCredentialsHelper = true
	}
	return a
}

func (a *Assembler) SetTerraformImage(image string) *Assembler {
	a.TerraformImage = image
	return a
}

func (a *Assembler) SetBusyboxImage(image string) *Assembler {
	a.BusyboxImage = image
	return a
}

func (a *Assembler) SetGitImage(image string) *Assembler {
	a.GitImage = image
	return a
}

func (a *Assembler) SetGit(git types.Git) *Assembler {
	a.Git = git
	return a
}

func (a *Assembler) SetEnvs(envs []v1.EnvVar) *Assembler {
	a.Envs = envs
	return a
}
