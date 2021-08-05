# myshoes: Auto scaling self-hosted runner for GitHub Actions

![](./docs/img/myshoes_logo_yoko_colorA.png)

[![Go Reference](https://pkg.go.dev/badge/github.com/whywaita/myshoes.svg)](https://pkg.go.dev/github.com/whywaita/myshoes)
[![test](https://github.com/whywaita/myshoes/actions/workflows/test.yaml/badge.svg)](https://github.com/whywaita/myshoes/actions/workflows/test.yaml)
[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/whywaita/myshoes)](https://goreportcard.com/report/github.com/whywaita/myshoes)

Auto scaling self-hosted runner :runner: (like GitHub-hosted) for GitHub Actions!

# features

- Auto-scaling and runner with your cloud-provider
    - your infrastructure (private cloud, homelab...)
        - [LXD](https://linuxcontainers.org): [shoes-lxd](https://github.com/whywaita/myshoes-providers/tree/master/shoes-lxd)
        - [OpenStack](https://www.openstack.org): [shoes-openstack](https://github.com/whywaita/myshoes-providers/tree/master/shoes-openstack)
    - a low-cost instance in public cloud
        - [AWS EC2 Spot Instances](https://aws.amazon.com/ec2/spot): [shoes-aws](https://github.com/whywaita/myshoes-providers/tree/master/shoes-aws)
        - [GCP Preemptible VM instances](https://cloud.google.com/compute/docs/instances/preemptible): shoes-gcp (not yet)
    - using special hardware
        - Graphics Processing Unit (GPU)
        - Field Programmable Gate Array (FPGA)

## Setup (only once)

Please check [setup.md](./docs/setup.md)

## Known issue

- sometimes occurred race condition if running multi jobs
  - GitHub Action's runner has a problem like a race condition.
  - related PullRequest: https://github.com/actions/runner/pull/660
