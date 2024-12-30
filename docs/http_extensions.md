# HTTP Extensions

## HTTP Plugin

> If needed, this plugin can be disabled with `KCLX_HTTP_PLUGIN_DISABLED=true`.

Alternative HTTP plugin to [kcl-lang/kcl-plugin](https://github.com/kcl-lang/kcl-plugin), which can be used to GET external resources. This one uses plain `net/http`. E.g.:

`http.get("https://example.com", timeout="10s")` -> `{"body": "<...>", "status": 200}`

You can parse the body using one of KCL's native functions e.g. `json.decode` or `yaml.decode`.
