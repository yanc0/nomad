package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type VarReadCommand struct {
	Meta
}

func (c *VarReadCommand) Help() string {
	helpText := `
Usage: nomad var read [options] <path>

  Read is used to get the contents of an existing secure variable.

  If ACLs are enabled, this command requires a token with the 'var:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Read Options:

  -json
    Output the secure variables in JSON format.

  -t
    Format and display the secure variables using a Go template.
  
`
	return strings.TrimSpace(helpText)
}

func (c *VarReadCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		},
	)
}

func (c *VarReadCommand) AutocompleteArgs() complete.Predictor {
	return SecureVariablePathPredictor(c.Meta.Client)
}

func (c *VarReadCommand) Synopsis() string {
	return "Read a secure variable"
}

func (c *VarReadCommand) Name() string { return "var read" }

func (c *VarReadCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <path>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	path := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	sv, _, err := client.SecureVariables().Read(path, api.WithNamespace(c.Meta.namespace))
	if err != nil {
		if err.Error() == "secure variable not found" {
			c.Ui.Warn(fmt.Sprint("Secure variable not found"))
			return 0
		}
		c.Ui.Error(fmt.Sprintf("Error retrieving secure variable: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, sv)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	meta := []string{
		fmt.Sprintf("Namespace|%s", sv.Namespace),
		fmt.Sprintf("Path|%s", sv.Path),
		fmt.Sprintf("Create Time|%v", formatTime(sv.CreateTime)),
	}
	if sv.CreateTime != sv.ModifyTime {
		meta = append(meta, fmt.Sprintf("Modify Time|%v", formatTime(sv.ModifyTime)))
	}
	meta = append(meta, fmt.Sprintf("Check Index|%v", sv.ModifyIndex))
	c.Ui.Output(formatKV(meta))

	c.Ui.Output(c.Colorize().Color("\n[bold]Items[reset]"))
	c.Ui.Output(formatItems(sv))
	return 0
}

func formatItems(sv *api.SecureVariable) string {
	items := make(svItems, 0, len(sv.Items))
	for k, v := range sv.Items {
		items = append(items, svItem{k, v})
	}

	// Sort the output by the item key
	sort.Sort(items)

	rows := make([]string, len(items))
	for i, item := range items {
		rows[i] = item.String()
	}
	return formatKV(rows)
}

type svItems []svItem

type svItem struct {
	k string
	v string
}

func (vi svItems) Len() int {
	return len(vi)
}

func (vi svItems) Less(i, j int) bool {
	return strings.ToLower(vi[i].k) < strings.ToLower(vi[j].k)
}

func (vi svItems) Swap(i, j int) {
	vi[i], vi[j] = vi[j], vi[i]
}

func (vi svItem) String() string {
	return fmt.Sprintf("%s|%s", vi.k, vi.v)
}
