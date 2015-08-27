# Hammer

Hammer takes YAML specs and uses them to build packages (with
[FPM](https://github.com/jordansissel/fpm).) Here's a fairly complete sample
annotated with comments.

```yaml
---
# the following basic attributes can be used in templates uppercased - so "name"
# becomes "{{.Name}}", "url" becomes "{{.URL}}"
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
  - src: "{{.Root}}/consul.service"
    dest: /etc/systemd/system/consul.service
  - src: "{{.Root}}/consul.json"
    dest: /etc/consul/
    config: true
  - src: "{{.Root}}/consul-ui.json"
    dest: /etc/consul/
    config: true
  - src: "{{.Root}}/consul.sysconfig"
    dest: /etc/sysconfig/consul
    config: true

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

# extra options to FPM for building RPMs. Other package support (deb, for
# example) is not currently supported but not terribly hard to add. Open an
# issue if you want it.
rpm:
    os: linux
    dist: CentOS
```

When saved as `spec.yml` in some folder, Hammer will find and execute it,
producing a package. Most fields are templated, and you can use Go templates to
get fields on the
[Package](https://godoc.org/github.com/asteris-llc/hammer#Package) struct.

## Installation

First, you'll need to get [FPM](https://github.com/jordansissel/fpm) (which
should just be `gem install fpm`) and any build tools you need (for example, to
build RPMs you'll need `rpmbuild`.) Then, `go install
github.com/asteris-llc/hammer`.
