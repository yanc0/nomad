package command

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type VarApplyCommand struct {
	Meta
	checkIndex  bool
	contents    *[]byte
	newVariable *api.SecureVariable
}

func (c *VarApplyCommand) Help() string {
	helpText := `
Usage: nomad var apply [options] <path>

  Apply is used to create or update an existing secure variable.

  If ACLs are enabled, this command requires a token with the 'var:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Apply Options:

  -var 'key=value'
    Variable for template, can be used multiple times.

  -var-file=path
    Path to HCL2 file containing user variables.
  
`
	return strings.TrimSpace(helpText)
}

func (c *VarApplyCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-check-index": complete.PredictNothing,
		},
	)
}

func (c *VarApplyCommand) AutocompleteArgs() complete.Predictor {
	return SecureVariablePathPredictor(c.Meta.Client)
}

func (c *VarApplyCommand) Synopsis() string {
	return "Create or update a secure variable"
}

func (c *VarApplyCommand) Name() string { return "var apply" }

func (c *VarApplyCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&c.checkIndex, "check-index", false, "enforce the modify index of the object; defaults to 0")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one argument
	args = flags.Args()
	if l := len(args); l > 1 {
		c.Ui.Error("This command takes zero or one argument: <path>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	path := args[0]
	c.newVariable = new(api.SecureVariable)

	// Read from stdin to see if there is a SecureVariable specification there.
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		contents, err := io.ReadAll(os.Stdin)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Unable to parse stdin as a secure variable: %s", err))
			return 1
		}
		err = json.Unmarshal(contents, c.newVariable)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Unable to parse stdin as a secure variable: %s", err))
			return 1
		}
	}
	if c.newVariable.Path == "" {
		c.newVariable.Path = args[0]
	} else {
		c.Ui.Warn("Using path from provided secure variable specification")
	}

	if c.newVariable.Namespace == "" {
		c.newVariable.Namespace = c.Meta.namespace
	} else {
		c.Ui.Warn("Using namespace from provided secure variable specification")
		c.Meta.namespace = c.newVariable.Namespace
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	var createFn func(*api.SecureVariable, *api.WriteOptions) (*api.WriteMeta, error)
	createFn = client.SecureVariables().Upsert
	if c.checkIndex {
		createFn = client.SecureVariables().UpsertWithCheckIndex
	}
	_, err = createFn(c.newVariable, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating secure variable: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully created secure variable %q!", path))
	return 0
}
