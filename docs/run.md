# Run Options

Hammer has three commands: `build`, `query`, and `help`.

<!-- markdown-toc start - Don't edit this section. Run M-x markdown-toc-generate-toc again -->
**Table of Contents**

- [Run Options](#run-options)
    - [global flags](#global-flags)
    - [`build` command](#build-command)
        - [flags](#flags)
    - [`query` command](#query-command)
    - [`help` command](#help-command)

<!-- markdown-toc end -->

## global flags

- `--log-format` defaults to "text", but can also be set to "json" for parseable
  JSON log output
- `--log-level` defaults to "info", which should provide any information you
  need. If you're building any tooling around Hammer, we'd recommend setting
  this to "debug" with the JSON log format. Valid values are "debug", "info",
  "warn", "error", and "fatal". Each successive value in that list provides less
  output.
- `--search` specifies the search directory for packages (for `build` and
  `query`.) To build somewhere else (in a CI system, for example) set this to
  the path with the root of your packages.

## `build` command

`hammer build` builds all packages it can find in the directory below the
current working directory by default. If you specify arguments (`hammer build
{package}`), they will be used as a filter on package names. So if you have
specs for package `first`, `second`, and `third`, `hammer build first third`
would only build the packages specified.

### flags

- `--cache` location of the cache on disk. It defaults to the current working
  directory, plus `.hammer-cache`. The cache directory will be created if it's
  not present.
- `--concurrent-jobs` the number of packages that will be built at once. This
  defaults to the number of CPU cores in the system.
- `--logs` location of build logs (stdout and stderr of build command) on disk.
  It defaults to the current working directory, plus `logs`. The logs directory
  will be created if it's not present.
- `--output` location of output packages on disk. It defaults to the current
  working directory, plus `out`. The output directory will be created if it's
  not present.
- `--shell` designates the command that will be used to run the build script. It
  will receive the location of the build script on disk as the first argument.

## `query` command

`hammer query {template}` will render the provided template for each package
found. It uses Go's templating lannguage, and the scope of `.` is the current
package.

Example usage: `hammer query '{{.Name}}: {{.Description}}'` will output
something like the following:

```
test: test package description
test2: test package two description
```

## `help` command

You can run `hammer help {topic}` for detailed help on any command.
