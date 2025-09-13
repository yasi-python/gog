package cli

import (
	"flag"
	"fmt"
)

type Command int

const (
	CmdRun Command = iota
	CmdStatus
	CmdReprobe
	CmdRollback
	CmdConfigTest
)

type Args struct {
	Cmd      Command
	Config   string
	ID       string
}

func Parse() Args {
	var (
		run    = flag.Bool("run", false, "run manager")
		status = flag.Bool("status", false, "print brief status")
		reprobe= flag.String("reprobe", "", "reprobe config id")
		rollback=flag.String("rollback","", "rollback config id")
		cfg    = flag.String("config", "config.yaml", "config path")
		test   = flag.Bool("config-test", false, "validate config")
	)
	flag.Parse()
	out := Args{Config: *cfg}
	switch {
	case *run:
		out.Cmd = CmdRun
	case *status:
		out.Cmd = CmdStatus
	case *reprobe != "":
		out.Cmd = CmdReprobe; out.ID = *reprobe
	case *rollback != "":
		out.Cmd = CmdRollback; out.ID = *rollback
	case *test:
		out.Cmd = CmdConfigTest
	default:
		fmt.Println("Use -run | -status | -reprobe <id> | -rollback <id> | -config-test")
		out.Cmd = CmdStatus
	}
	return out
}