# myshoes: Auto scaling self-hosted runner for GitHub Actions

![](./docs/assets/img/myshoes_logo_yoko_colorA.png)

[![awesome-runners](https://img.shields.io/badge/listed%20on-awesome--runners-blue.svg)](https://github.com/jonico/awesome-runners)
[![Go Reference](https://pkg.go.dev/badge/github.com/whywaita/myshoes.svg)](https://pkg.go.dev/github.com/whywaita/myshoes)
[![test](https://github.com/whywaita/myshoes/actions/workflows/test.yaml/badge.svg)](https://github.com/whywaita/myshoes/actions/workflows/test.yaml)
[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/whywaita/myshoes)](https://goreportcard.com/report/github.com/whywaita/myshoes)

Auto scaling self-hosted runner :runner: (like GitHub-hosted) for GitHub Actions!

## Features

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
    - And more in [whywaita/myshoes-providers](https://github.com/whywaita/myshoes-providers)

## Setup (only once)

Please see [Documents](./docs).

## How to contribute

1. Fork it
1. Clone original repository `git clone https://github.com/whywaita/myshoes`
1. Add remote your repository `git remote add your-name https://github.com/${your-name}/myshoes`
1. Create your feature branch `git switch -c my-new-feature`
1. Commit your changes `git commit -am 'Add some feature'`
1. Push to the branch `git push your-name my-new-feature`
1. Create new Pull Request

## Publications

### Talk

- [Development myshoes and Provide Cycloud-hosted runner -- GitHub Actions with your shoes. (en)](https://www.slideshare.net/whywaita/development-myshoes-and-provide-cycloudhosted-runner-github-actions-with-your-shoes)
- [Development OSS CI/CD platform in CyberAgent (ja)](https://www.slideshare.net/whywaita/cyberagent-oss-cicd-myshoes-cicd2021)
