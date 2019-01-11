package main

import (
	"fmt"

	"code.cloudfoundry.org/cli/plugin"
)

func (c *PaaSDronePlugin) DeployDroneServer(cliConnection plugin.CliConnection, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("Not enough inputs provided. See `cf help` for info.")
	}
	err := c.CreateDroneDB(cliConnection)
	if err != nil {
		return err
	}
	droneFqdn, err := c.CreateDroneServer(cliConnection, args[0], args[1], args[2])
	if err != nil {
		return err
	}

	err = c.CreateDroneAgent(cliConnection, fmt.Sprintf("https://%v", droneFqdn), args[2])
	if err != nil {
		return err
	}
	return err
}

func (c *PaaSDronePlugin) DestroyDroneServer(cliConnection plugin.CliConnection, args []string) error {
	cliConnection.CliCommand("delete", "drone")
	cliConnection.CliCommand("delete-service", "drone-agent")
	cliConnection.CliCommand("delete-service", "drone-db")
	return nil
}
