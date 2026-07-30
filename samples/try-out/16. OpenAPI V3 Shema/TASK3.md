We recently added openAPIV3Schema support for component types, traits and workflows. It will be the default schema for all our samples. So we need to move all the ocSchema related samples to a new folder called
ocSchema. docs/templating/openapiv3-schema.md

Move all the ocSchema related samples in the following folders to "samples/ocSchema/" with naming such as "samples/ocSchema/workflows/".
- samples/getting-started/component-traits/
- samples/getting-started/workflows/
- samples/getting-started/component-types/

Convert every other samples from ocSchema to openAPIV3Schema.
- Remove: samples/component-alerts/alert-rule-trait.yaml, rename and remove suffix openapiv3: samples/component-alerts/alert-rule-trait-openapiv3.yaml
- Update README: samples/component-alerts/README.md

- Remove: samples/component-types/component-with-api-management/component-with-api-management.yaml, rename and remove suffix openapiv3: samples/component-types/component-with-api-management/component-with-api-management-openapiv3.yaml
- Update README: samples/component-types/component-with-api-management/README.md

- Remove: samples/component-types/component-with-configs/component-with-configs.yaml, rename and remove suffix openapiv3: samples/component-types/component-with-configs/component-with-configs-openapiv3.yaml
- Update README: samples/component-types/component-with-configs/README.md

- Remove: samples/component-types/component-with-embedded-traits/component-with-embedded-traits.yaml, rename and remove suffix openapiv3: samples/component-types/component-with-embedded-traits/component-with-embedded-traits-openapiv3.yaml
- Update README: samples/component-types/component-with-embedded-traits/README.md

- Remove: samples/getting-started/component-traits/alert-rule-trait.yaml, rename and remove suffix openapiv3: samples/getting-started/component-traits/alert-rule-trait-openapiv3.yaml

- Update from ocSchema to openAPIV3Schema: samples/getting-started/component-traits/api-management.yaml

- Remove all ocSchema related samples: samples/getting-started/component-types/
- Rename and remove suffix openapiv3: samples/getting-started/component-types/

- Remove all ocSchema related samples: samples/getting-started/workflows/
- Rename and remove suffix openapiv3: samples/getting-started/workflows/

- Update all.yaml: samples/getting-started/all.yaml
- If needed update: samples/getting-started/README.md

- Remove ocSchema related samples and rename and remove suffix openapiv3 from all the folders inside the following folder: samples/gitops-workflows/*
- Update READMEs if necessary.

- Remove ocSchema related samples and rename and remove suffix openapiv3 from all the folders inside the following folder: samples/workflows/*
- Update READMEs if necessary.
