package init_container

// Assembler helps to assemble the init containers
type Assembler struct {
	Name string

	TerraformCredential        bool
	TerraformRC                bool
	TerraformCredentialsHelper bool
}
