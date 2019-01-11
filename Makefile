build:
	go build -o paas-drone_cf_cli_plugin paas-drone_cf_cli_plugin.go commands.go operations.go
	cf install-plugin -f paas-drone_cf_cli_plugin
