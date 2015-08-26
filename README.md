# Hammer

Hammer takes YAML specs and uses them to build packages (with
[FPM](https://github.com/jordansissel/fpm).) Here's a sample:

    ---
    name: consul
    version: 0.5.2
    iteration: 1
    epoch: 1
    vendor: Hashicorp
    url: https://consul.io
    description: Service discovery and configuration made easy. Distributed, highly available, and datacenter-aware.

    depends: []

    resources:
      - url: https://dl.bintray.com/mitchellh/consul/{{.Version}}_linux_amd64.zip
        hash-type: sha1
        hash: b3ae610c670fc3b81737d44724ebde969da66ebf

    targets:
      - src: "{{.BuildRoot}}/consul"
        dest: /usr/bin/consul

    scripts:
      build: |
        unzip {{.Version}}_linux_amd64.zip

      before-install: |
        echo script before installing {{.Name}}:{{.Version}}

      after-install: |
        echo script after installing {{.Name}}:{{.Version}}

      before-remove: |
        echo script before removing {{.Name}}:{{.Version}}

      after-remove: |
        echo script after removing {{.Name}}:{{.Version}}

      before-upgrade: |
        echo script before upgrading {{.Name}}:{{.Version}}

      after-upgrade: |
        echo script after upgrading {{.Name}}:{{.Version}}

When saved as `spec.yml` in some folder, Hammer will find and execute it,
producing a package. Most fields are templated, and you can use Go templates to
get fields on the
[Package](https://godoc.org/github.com/asteris-llc/hammer#Package) struct.

## Installation

First, you'll need to get [FPM](https://github.com/jordansissel/fpm) (which
should just be `gem install fpm`) and any build tools you need (for example, to
build RPMs you'll need `rpmbuild`.) Then, `go install
github.com/asteris-llc/hammer`.
