package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
)

type EnvVar struct {
	Name  string
	Value string
}

type DatasourceResponse struct {
	SystemEnvJSON struct {
		VCAPSERVICES struct {
			Postgres []struct {
				Credentials struct {
					URI string `json:"uri"`
				} `json:"credentials"`
			} `json:"postgres"`
		} `json:"VCAP_SERVICES"`
	} `json:"system_env_json"`
}

func (c *PaaSDronePlugin) CheckServiceState(cliConnection plugin.CliConnection, serviceName string) error {
	gso, _ := cliConnection.GetService(serviceName)
	if gso.LastOperation.Type == "create" {
		if gso.LastOperation.State == "succeeded" {
			return nil
		} else if gso.LastOperation.State == "in progress" {
			return fmt.Errorf("A service with name '%v' already exists, and is being created\nCheck the progress with `cf service %v` and retry once completed.", serviceName, serviceName)
		}
	} else if gso.LastOperation.Type == "delete" {
		return fmt.Errorf("A service with name '%v' already exists, but is being deleted\nCheck the progress with `cf service %v` and retry once completed.", serviceName, serviceName)
	}
	return fmt.Errorf("You shouldn't really have gotten to this point!")
}

func (c *PaaSDronePlugin) SetAppEnvVars(cliConnection plugin.CliConnection, appName string, envVar EnvVar) error {
	cliCommand := []string{
		"set-env",
		appName,
		envVar.Name,
		envVar.Value,
	}
	output, err := cliConnection.CliCommandWithoutTerminalOutput(cliCommand...)
	if err != nil {
		return fmt.Errorf("%v", strings.Join(output, ""))
	}
	return err
}

func (c *PaaSDronePlugin) CreateDroneDB(cliConnection plugin.CliConnection) error {
	_, err := cliConnection.GetService("drone-db")
	if err != nil {
		fmt.Println("Creating a database instance")
		_, err := cliConnection.CliCommandWithoutTerminalOutput("create-service", "postgres", "tiny-unencrypted-9.5", "drone-db")
		if err != nil {
			return fmt.Errorf("Database creation failed: %v", err)
		}
	}
	return c.CheckServiceState(cliConnection, "drone-db")
}

func (c *PaaSDronePlugin) CreateDroneAgent(cliConnection plugin.CliConnection, serverUrl string, serverSecret string) error {
	_, err := cliConnection.GetService("drone-agent")
	if err != nil {
		fmt.Println("Creating a drone agent instance")
		agentCli := []string{"create-service", "drone-agent", "t2.nano", "drone-agent", "-c", fmt.Sprintf("{\"server_address\":\"%v\",\"server_secret\":\"%v\"}", serverUrl, serverSecret)}
		_, err := cliConnection.CliCommandWithoutTerminalOutput(agentCli...)
		if err != nil {
			return fmt.Errorf("Database creation failed: %v", err)
		}
	}
	return c.CheckServiceState(cliConnection, "drone-agent")
}

func (c *PaaSDronePlugin) CreateDroneServer(cliConnection plugin.CliConnection, gitHubClientId string, gitHubClientSecret string, serverSecret string) (url string, err error) {
	_, err = cliConnection.GetApp("drone")
	if err != nil {
		fmt.Println("Creating a drone server instance")
		agentCli := []string{
			"push",
			"drone",
			"--docker-image=drone/drone:1.0.0-rc.1",
			"--health-check-type=process",
			"--no-start",
		}
		_, err := cliConnection.CliCommand(agentCli...)
		if err != nil {
			return "", fmt.Errorf("Drone Server creation failed: %v", err)
		}
	}
	_, err = cliConnection.CliCommand("bind-service", "drone", "drone-db")
	if err != nil {
		return "", fmt.Errorf("Binding of Database to Drone Server failed.")
	}
	fmt.Println("Setting Drone Server environment variables")
	gao, _ := cliConnection.GetApp("drone")
	datasourceJson, _ := cliConnection.CliCommandWithoutTerminalOutput("curl", fmt.Sprintf("/v2/apps/%v/env", gao.Guid))
	var datasource DatasourceResponse
	err = json.Unmarshal([]byte(strings.Join(datasourceJson, "\n")), &datasource)
	if err != nil {
		return "", err
	}
	envVars := []EnvVar{
		EnvVar{
			Name:  "DRONE_GITHUB_SERVER",
			Value: "https://github.com",
		},
		EnvVar{
			Name:  "DRONE_GITHUB_CLIENT_ID",
			Value: gitHubClientId,
		},
		EnvVar{
			Name:  "DRONE_GITHUB_CLIENT_SECRET",
			Value: gitHubClientSecret,
		},
		EnvVar{
			Name:  "DRONE_RUNNER_CAPACITY",
			Value: "0",
		},
		EnvVar{
			Name:  "DRONE_SERVER_HOST",
			Value: fmt.Sprintf("%v.%v", gao.Routes[0].Host, gao.Routes[0].Domain.Name),
		},
		EnvVar{
			Name:  "DRONE_SERVER_PROTO",
			Value: "https",
		},
		EnvVar{
			Name:  "DRONE_RPC_SECRET",
			Value: serverSecret,
		},
		EnvVar{
			Name:  "DRONE_DATABASE_DRIVER",
			Value: "postgres",
		},
		EnvVar{
			Name:  "DRONE_DATABASE_DATASOURCE",
			Value: datasource.SystemEnvJSON.VCAPSERVICES.Postgres[0].Credentials.URI,
		},
	}

	for _, envVar := range envVars {
		c.SetAppEnvVars(cliConnection, "drone", envVar)
	}
	cliConnection.CliCommandWithoutTerminalOutput("restage", "drone")
	fmt.Println("Starting Drone Server")
	cliConnection.CliCommand("restart", "drone")

	// fmt.Println(gao)
	return fmt.Sprintf("%v.%v", gao.Routes[0].Host, gao.Routes[0].Domain.Name), nil
}
