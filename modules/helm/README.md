# kclx/helm

```sh
kcl mod add oci://ghcr.io/macropower/kclx/helm
```

## Values Schema

You can optionally use a schema for a `helm.Chart`'s `values`. This schema can be imported from a Helm Chart's `values.schema.json` file if one is available, or alternatively it can be generated from one or more `values.yaml` files.

If you do this, you should automate it in some way, so that you can keep the values schema up-to-date with the chart.

### Setup

NOTE: This is assuming that you will likely have some charts that include a jsonschema, and others that do not.

First, install the `helm schema` plugin. This is a fork which has been modified to work better with `kcl import`.

```bash
helm plugin install https://github.com/MacroPower/helm-values-schema-json.git
```

Next, for each chart where a values.schema.json file must be generated, create a `.schema.yaml` file. This is used to configure the `helm schema` plugin. Specify one or more URLs pointing to example values.yaml files for the chart.

```yaml
## Files used to infer the jsonschema of the values.yaml file, which is used to
## generate the KCL schema for the chart values. Multiple inputs can be defined
## to provide additional data for schema inference.
##
input:
  - https://example.com/charts/example/values.yaml

schemaRoot:
  ## Defines the name of the root KCL schema.
  ##
  title: Values

schema:
  ## Setting `additionalProperties: true` will add `[...str]: any` to all
  ## objects in the KCL schema, which is necessary for defining any values not
  ## included in the default values.yaml file.
  ##
  additionalProperties: true
```

Now, generate a jsonschema file. Alternatively, if there is an official values.schema.json file available for the chart, download it directly. This should be called `values.schema.json`.

```bash
helm schema
# OR
curl -o values.schema.json https://example.com/charts/example/values.schema.json
```

Finally, convert the jsonschema file into KCL schemas, and remove `values.schema.json` as it will no longer be needed.

```bash
kcl import -m jsonschema values.schema.json --force
rm values.schema.json
```

The `kcl import` command will generate a file `values.schema.k` with a root schema called `Values`. You can use this schema in your KCL code to get completion and validation for any data passed to the `values` argument.

```py
import helm

helm.template(helm.Chart {
  chart = "example"
  targetRevision = "0.1.0"
  repoURL = "https://example.com/charts"
  values = Values { # <- Uses the Values schema from values.schema.k
    replicas: 3
  }
})
```

If you're forced to generate a schema, this won't be perfect, since `helm schema` is just doing its best to infer the schema from the union of all inputs. Using `additionalProperties: true` will allow you to drift from the schema somewhat by allowing extra fields to be added to any schema. This is useful if some input you need to use was not present in the example values.yaml file, and thus was not added to the schema.

If you happen to find a chart that does actually have a full values.schema.json file (or uses a common library which has one, like [bjw-s/common](https://github.com/bjw-s/helm-charts)), it will produce much better results, but unfortunately I have found that not many charts include a jsonschema.
