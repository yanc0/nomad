package allocrunner

import (
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
)

var (
	_ interfaces.RunnerPrerunHook  = (*checksHook)(nil)
	_ interfaces.RunnerUpdateHook  = (*checksHook)(nil)
	_ interfaces.RunnerPreKillHook = (*checksHook)(nil)
)
