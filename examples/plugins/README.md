# Plugin examples

Build the sample provider:

```sh
go build -o devx-provider-echo ./examples/plugins/devx-provider-echo
```

Protocol:

- `devx-provider-echo describe` prints JSON metadata.
- `devx-provider-echo render` prints a compose fragment as JSON or YAML (empty in this sample).
