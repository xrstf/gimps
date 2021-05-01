# go-imports-sorter

This is a fork of [goimports-reviser](https://github.com/incu6us/goimports-reviser). The purpose of
forking was to replace the fixed import classes ("std", "local", "project", ...) with a configurable
set of classes, so that the order can be configured much more fine-grained.

For example, in Gardener the logic is to group not just imports from the same package, but from the
same Github organization, like

```
import (
	"os"

	"github.com/gardener/gardener/pkg/bla"
	"github.com/gardener/gardener-extension-provider-aws/pkg/foo"

	"sigs.k8s.io/controller-runtime/cache"
)
```

This rule was impossible to do with goimports-reviser, but this project can do it via a config file
like so:

```yaml
importOrder: [std, gardener, external]
sets:
  - name: gardener
    patterns:
      - 'github.com/gardener/*'
```

The configuration file is now also where all the flags have been moved. Run it like so:

```bash
go-imports-sorter [-stdout] -config CONFIG_FILE.yaml FILE[, ...]
```

Most of the README for goimports-reviser is still true for this fork.
