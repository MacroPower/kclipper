# OS Extensions

## OS Plugin

> If needed, this plugin can be disabled with `KCLX_OS_PLUGIN_DISABLED=true`.

Run a command on the host OS. This can be useful for integrating with other tools that do not have a native KCL plugin available, e.g. by installing them in your container. E.g.:

`os.exec("command", ["arg"])` -> `{"stdout": "x", "stderr": "y"}`

You can parse stdout using one of KCL's native functions e.g. `json.decode` or `yaml.decode`.
