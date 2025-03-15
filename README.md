# Migration note

> [!IMPORTANT]
> `gimps` has been migrated to [codeberg.org/xrstf/gimps](https://codeberg.org/xrstf/gimps).

---

## gimps, the Go IMPort Sorter

This is a fork of [goimports-reviser](https://github.com/incu6us/goimports-reviser). The purpose of
forking was to replace the fixed import classes ("std", "local", "project", ...) with a configurable
set of classes, so that the order can be configured much more fine-grained.

### Example

At [Kubermatic](https://kubermatic.com/) imports are grouped in 4 groups:

- standard library
- external packages that are not in the following two groups
- kubermatic packages
- kubernetes packages

`kubermatic` here means both `k8c.io/*` as well as `github.com/kubermatic/*`, whereas `kubernetes`
includes `k8s.io/*` and `*.k8s.io`.

This setup can be configured in gimps like so:

```yaml
importOrder: [std, external, kubermatic, kubernetes]
sets:
  - name: kubermatic
    patterns:
      - 'k8c.io/**'
      - 'github.com/kubermatic/**'
  - name: kubernetes
    patterns:
      - 'k8s.io/**'
      - '*.k8s.io/**'
```

Note that `std` and `external` are pre-defined by gimps and cannot be configured explicitly.

Then running `gimps -config configfile.yaml .` will automatically fix all Go files, except for
the `vendor` folder and generated files.

### Changes in this Fork

- Output is always formatted, `-format` has been removed.
- The notion of `local` packages with configurable prefixes has been removed,
  configure a set instead. Usually the `project` set will be sufficient.
- Likewise, the notion of "version aliases" was replaced with a flexible rule system,
  allowing to configure dynamic aliases for matching packages and rewriting the code
  accordingly (see below for important notes on this).
- Configuration happens primarily via the config file `.gimps.yaml`.
- Many files and directories can be specified as one; the main focus of gimps is not to be
  a goimports/gopls alternative, but to be an addition and useful in CI environments.
- `github.com/owner/repos-are-great` is not considered a local/project package of a
  `github.com/owner/repo` module (stricter check for prefix).
- gimps uses [goimports-reviser](https://github.com/incu6us/goimports-reviser)
  code for the AST parsing, but large chunks of the `reviser` package have been rewritten.

### Installation

```bash
go install go.xrstf.de/gimps
```

Alternatively, you can download the [latest release](https://codeberg.org/xrstf/gimps/releases/latest) from Codeberg.

### Configuration

```
Usage of gimps:
  -c, --config string   Path to the config file (mandatory).
  -d, --dry-run         Do not update files.
  -s, --stdout          Print output to stdout instead of updating the source file(s).
  -v, --verbose         List all instead of just changed files.
  -V, --version         Show version and exit.
```

gimps uses a `.gimps.yaml` file that can either be given explicitly via `-config FILE.yaml` or
it can be placed in the Go module root (where your `go.mod` lives) and must then be named
`.gimps.yaml`.

The configuration is rather simple:

```yaml
# By default, gimps detects the project name based on the go.mod file.
# If this fails or you don't have a go.mod file, you can configure the
# name here.
projectName: github.com/example/repo

# This list is the order of import sets in the output of each file.
#
#   - `std` is predefined and represents all Go standard library packages
#   - `external` is predefined and represents all packages that do not
#     match any of the other sets.
#   - `project` is predefined and presents packages in the same project
#     (i.e. have the project name as their prefix)
#
# The default order is shown below. If you define more sets (see below),
# add them to this list in the spot where the matching imports should be
# placed.
#
# Important: If you define a set and not use it in the importOrder, the
#            imports that match the set's patterns will be dropped!
importOrder: [std, project, external]

# Define additional groups of imports. Their names are then used in the
# importOrder above.
sets:
  - # a unique name
    name: kubermatic
    # a list of glob-expressions, with the addition that double star
    # expressions are allowed (`foo/**` matches `foo/bar/bar`)
    patterns:
      - 'k8c.io/**'
      - 'github.com/kubermatic/**'

  - name: kubernetes
    patterns:
      - 'k8s.io/**'
      - '*.k8s.io/**'

# gimps can enforce aliases for certain imports. For example, you can ensure
# that all imports of "k8s.io/api/core/v1" are aliased as "corev1".
# Rules are processed one at a time and the first matching is applied.
#
# There are a few important caveats to note:
#
#   * gimps only tokenizes source code and doesn't have deep knowledge of
#     the semantics. If a package named "foo" is important and a local
#     function call to `foo.DoSomething()` happens, gimps cannot determine
#     whether "foo" here is the package or a local variable.
#     To prevent accidental rewrites, ensure to never name a variable after
#     any imported package (i.e. don't shadow the package name).
#   * Rewriting aliases requires to load package dependencies for each
#     package that is processed. This requires quite a bit of CPU and can
#     can slow down gimps noticibly. If no rules are configured, gimps
#     automatically skips loading package dependencies.
aliasRules:
  - # a unique name
    name: k8s-api
    # a Go regexp, ideally anchored (with ^ and $) to prevent mismatches,
    # all packages that match will get an alias as configured below.
    # The example below matches for example "k8s.io/api/core/v1"
    expr: '^k8s.io/api/([a-z0-9-]+)/(v[a-z0-9-]+)$'
    # the alias to use for the import; you will most likely always use
    # references to groups in the expr ($1 gives the first matched group, etc.).
    # Pay attention to not accidentically generate the same alias for
    # multiple packages used in the same file (gimps will abort in this case).
    # With the example package above, the configuration below yields "corev1".
    alias: '$1$2'

  - name: k8s-apimachinery
    # contains an optional third subpackage, $4 will be empty if no subpackage was found
    expr: '^k8s.io/apimachinery/pkg/apis/([a-z0-9-]+)/(v[a-z0-9-]+)(/([a-z0-9-]+))?$'
    alias: '$1$2$4'

# paths that match any of the following glob expressions (relative to the
# go.mod) will be ignored; if nothing is configured, the following shows
# the default configuration.
exclude:
  - "vendor/**"
  - ".git/**"
  - "_build/**"
  - "node_modules/**"
  - "**/zz_generated.**"
  - "**/zz_generated_**"
  - "**/generated.pb.go"
  - "**/generated.proto"
  - "**/*_generated.go"

# whether or not to detect generated files by their content, this means
# checking if a top-line comment containing `(been generated|generated by|do not edit)`
# exists _before_ the package declaration
detectGeneratedFiles: true

# whether or not to remove unused imports; usually this is not needed,
# as your editor's Go integration, like gopls, takes care of that already.
removeUnusedImports: false

# transform each import with a version suffix into a more readable import,
# i.e. `"github.com/bmatcuk/doublestar/v4"` => `doublestar "github.com/bmatcuk/doublestar/v4"`
setVersionAlias: true
```

### Running

Put the configuration either in a `.gimps.yaml` in the module root (recommended) or configure
it explicitly via `-config`.

Provide one or more arguments, each being either a file or a directory. Directories are
automatically traversed recursively, except for the items noted in the example configuration above.

**Important:** The first argument controls the Go module path for the entire operation. It's not
recommended to make gimps work across multiple modules at the same time. Usually you want to
either run it from your module root directory or give it a single file explicitly to facilitate
editor integration when needed.

For the editor integration, you can specify `-stdout` to print the formatted file to stdout. This
only makes sense if you provide exactly one file, otherwise separating the output is difficult.

If you just want to see which files would be fixed, run with `-dry-run`.

Give `-verbose` to show all files being processed instead of just fixed files.

```bash
$ cd ~/myproject
$ gimps .
```

### License

The original reviser code is MIT licensed and (c) 2020 Vyacheslav Pryimak.

The new parts in this fork are MIT licensed and (c) 2021 Christoph Mewes.
