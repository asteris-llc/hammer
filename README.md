# Hammer

[![Build Status](https://travis-ci.org/asteris-llc/hammer.svg)](https://travis-ci.org/asteris-llc/hammer)
[![Documentation Status](https://readthedocs.org/projects/hammer/badge/?version=latest)](http://hammer.readthedocs.org/en/latest/?badge=latest)

Hammer takes YAML specs and uses them to build packages (with
[FPM](https://github.com/jordansissel/fpm).) Here's a pretty minimal example,
please see [the docs](http://hammer.readthedocs.org/) for more information.

``` yaml
---
name: consul-cli
version: 0.1.1
license: APL 2.0
iteration: 2
vendor: Cisco
url: https://github.com/CiscoCloud/consul-cli
architecture: x86_64
description: Command line interface to Consul HTTP API
type: rpm
obsoletes:
  - consul-utils

resources:
  - url: https://github.com/CiscoCloud/consul-cli/releases/download/v{{.Version}}/consul-cli_{{.Version}}_linux_amd64.tar.gz
    hash-type: sha256
    hash: 1bc31fa70a9508a2c302bbdbd3073601bcad82694af8370d968c69c32471ee7f

targets:
  - src: '{{.BuildRoot}}/consul-cli_{{.Version}}_linux_amd64/consul-cli'
    dest: /usr/bin/

scripts:
  build: |
    tar -xzvf consul-cli_{{.Version}}_linux_amd64.tar.gz

extra-args: |
  --rpm-os linux
```

When saved as `spec.yml` in some folder, Hammer will find and execute it,
producing a package. Most fields are templated, and you can use Go templates to
get fields on the
[Package](https://godoc.org/github.com/asteris-llc/hammer/hammer#Package) struct.

## Installation

First, you'll need to get [FPM](https://github.com/jordansissel/fpm) (which
should just be `gem install fpm`) and any build tools you need (for example, to
build RPMs you'll need `rpmbuild`.) Then, `go install
github.com/asteris-llc/hammer`.

## Contributing

Got a feature you'd like to see? PRs are very welcome. Just make sure that your
build passes all the checks on
[gometalinter](https://github.com/alecthomas/gometalinter). Tests are
appreciated, as well!
