This page assumes your familiarity with the basics of building RPM or deb
packages.

The following describes the format that Hammer parses your YAML spec file into.
It is given in the form of an annotated Go struct from the Hammer source code,
as well as a sample spec file for building Consul. Together, they give a pretty
complete picture of the options available.

```go
// Package is the main struct in Hammer. It contains all the (meta-)information
// needed to produce a package.
type Package struct {
    Architecture string     // target processor architecture, e.g. x86_64
    Depends      []string   // runtime dependencies
    Description  string     // short package description
    Epoch        string     // strictly increasing package version
    ExtraArgs    string
    Attrs        []Attr     // RPM File attributes (%attr)
    Iteration    string
    License      string     // package license, e.g. MIT, APLv2, BSD
    Name         string
    Obsoletes    []string   // other packages that are obsoleted by this one
    Resources    []Resource // see example spec for details
    Scripts      Scripts    // see example spec for details
    Targets      []Target   // see example spec for details
    Type         string     // for now, must be RPM. deb support on the way
                            // (tracking in asteris-llc/hammer#23)
    URL          string     // project homepage
    Vendor       string     // who created the project?
    Version      string     // major.minor.patch, e.g. v0.1.5

    // ...

	Vars         map[string]string // dict of extra vars available to templates

    // ...
}
```

```yaml
---
# the following basic attributes can be used in templates uppercased - so "name"
# becomes "{{.Name}}", with one exception: "url" becomes "{{.URL}}"
name: consul
version: 0.5.2
license: MPLv2.0
iteration: 1
epoch: 1
vendor: Hashicorp
url: https://consul.io
architecture: x86_64
description: Consul is a tool for service discovery and configuration.

# list of targets that the built package will depend on
depends:
- systemd

# a list of resources (this can be source, but in this case is prebuilt
# binaries.) The URLs in this list can use template variables.
resources:
- url: https://dl.bintray.com/mitchellh/consul/{{.Version}}_linux_amd64.zip
    hash-type: sha1
    hash: b3ae610c670fc3b81737d44724ebde969da66ebf
- url: https://dl.bintray.com/mitchellh/consul/{{.Version}}_web_ui.zip
    hash-type: sha1
    hash: 67a2665e3c6aa6ca95c24d6176641010a1002cd6

# targets that will be copied into the package after the build is successful.
# The sources and destinations here can use template variables, and the content
# of the files can be templated as well, by providing `template: true` to any of
# the targets. Targets can also be marked as configuration files with the
# `config: true` option.
targets:
- src: "{{.BuildRoot}}/consul"
    dest: /usr/bin/
- src: "{{.BuildRoot}}/dist/"
    dest: /usr/share/consul-ui/
- src: "{{.SpecRoot}}/consul.service"
    dest: /etc/systemd/system/consul.service
- src: "{{.SpecRoot}}/consul.json"
    dest: /etc/consul/
    config: true
- src: "{{.SpecRoot}}/consul-ui.json"
    dest: /etc/consul/
    config: true
- src: "{{.SpecRoot}}/consul.sysconfig"
    dest: /etc/sysconfig/consul
    config: true

# This dictionary isn't necessary because we're not templating any of the above
# targets. If we were, we could include this in a template with the following
# expression: {{variable "packagedBy"}}. Variables and variable names can be any
# valid YAML string.
# vars:
#   packagedBy: 'username'

# scripts for building and installing the package. The only required script is
# "build", and "{before,after}-{install,remove,upgrade}" are available. You can
# also use template variables in the content of these scripts.
scripts:
build: |
    unzip {{.Version}}_linux_amd64.zip
    unzip {{.Version}}_web_ui.zip

before-install: |
    getent group consul > /dev/null || groupadd -r consul
    getent passwd consul > /dev/null || \
        useradd -r \
                -g consul \
                -d /var/lib/consul \
                -s /sbin/nologin \
                -c "consul.io user" \
                consul

    mkdir /var/lib/consul
    chown -R consul /var/lib/consul

after-install: |
    systemctl enable /etc/systemd/system/consul.service
    systemctl start consul.service

before-remove: |
    systemctl disable consul.service

after-remove: |
    rm -rf /var/lib/consul

after-upgrade: |
    systemctl reload-daemon
    systemctl restart consul.service

attrs:
  - file: /usr/bin/consul
    mode: 755
    user: consul
    group: consul

# extra options to FPM for building RPMs. Other package support (deb, for
# example) is not currently supported but not terribly hard to add. Open an
# issue if you want it.
rpm:
    os: linux
    dist: CentOS
```

For more examples, you can take a look at
[this repo](https://github.com/asteris-llc/mantl-packaging), which is also where
the above example was taken from!
