# KENGINR
<p>
	<a href="https://github.com/khulnasoft/kengine/actions/workflows/ci.yml"><img src="https://github.com/khulnasoft/kengine/actions/workflows/ci.yml/badge.svg"></a>
	<a href="https://pkg.go.dev/github.com/khulnasoft/kengine/v2"><img src="https://img.shields.io/badge/godoc-reference-%23007d9c.svg"></a>
	<br>
	<a href="https://twitter.com/khulnasoft" title="@khulnasoft on Twitter"><img src="https://img.shields.io/badge/twitter-@khulnasoft-55acee.svg" alt="@khulnasoft on Twitter"></a>
	<a href="https://kengine.community" title="Kengine Forum"><img src="https://img.shields.io/badge/community-forum-ff69b4.svg" alt="Kengine Forum"></a>
	<br>
	<a href="https://sourcegraph.com/github.com/khulnasoft/kengine?badge" title="Kengine on Sourcegraph"><img src="https://sourcegraph.com/github.com/khulnasoft/kengine/-/badge.svg" alt="Kengine on Sourcegraph"></a>
	<a href="https://cloudsmith.io/~kengine/repos/"><img src="https://img.shields.io/badge/OSS%20hosting%20by-cloudsmith-blue?logo=cloudsmith" alt="Cloudsmith"></a>
</p>
<p>
	<a href="https://github.com/khulnasoft/kengine/releases">Releases</a> ·
	<a href="https://khulnasoft.com/docs/">Documentation</a> ·
	<a href="https://kengine.community">Get Help</a>
</p>

### Menu

- [Features](#features)
- [Install](#install)
- [Build from source](#build-from-source)
  - [For development](#for-development)
  - [With version information and/or plugins](#with-version-information-andor-plugins)
- [Quick start](#quick-start)
- [Overview](#overview)
- [Full documentation](#full-documentation)
- [Getting help](#getting-help)
- [About](#about)

<p>
	<b>Powered by</b>
	<br>
	<a href="https://github.com/khulnasoft-lab/certmagic">
		<picture>
			<source media="(prefers-color-scheme: dark)" srcset="https://user-images.githubusercontent.com/55066419/206946718-740b6371-3df3-4d72-a822-47e4c48af999.png">
			<source media="(prefers-color-scheme: light)" srcset="https://user-images.githubusercontent.com/1128849/49704830-49d37200-fbd5-11e8-8385-767e0cd033c3.png">
			<img src="https://user-images.githubusercontent.com/1128849/49704830-49d37200-fbd5-11e8-8385-767e0cd033c3.png" alt="CertMagic" width="250">
		</picture>
	</a>
</p>

## [Features](https://khulnasoft.com/features)

- **Easy configuration** with the [Kenginefile](https://khulnasoft.com/docs/kenginefile)
- **Powerful configuration** with its [native JSON config](https://khulnasoft.com/docs/json/)
- **Dynamic configuration** with the [JSON API](https://khulnasoft.com/docs/api)
- [**Config adapters**](https://khulnasoft.com/docs/config-adapters) if you don't like JSON
- **Automatic HTTPS** by default
  - [ZeroSSL](https://zerossl.com) and [Let's Encrypt](https://letsencrypt.org) for public names
  - Fully-managed local CA for internal names & IPs
  - Can coordinate with other Kengine instances in a cluster
  - Multi-issuer fallback
- **Stays up when other servers go down** due to TLS/OCSP/certificate-related issues
- **Production-ready** after serving trillions of requests and managing millions of TLS certificates
- **Scales to hundreds of thousands of sites** as proven in production
- **HTTP/1.1, HTTP/2, and HTTP/3** all supported by default
- **Highly extensible** [modular architecture](https://khulnasoft.com/docs/architecture) lets Kengine do anything without bloat
- **Runs anywhere** with **no external dependencies** (not even libc)
- Written in Go, a language with higher **memory safety guarantees** than other servers
- Actually **fun to use**
- So much more to [discover](https://khulnasoft.com/features)

## Install

The simplest, cross-platform way to get started is to download Kengine from [GitHub Releases](https://github.com/khulnasoft/kengine/releases) and place the executable file in your PATH.

See [our online documentation](https://khulnasoft.com/docs/install) for other install instructions.

## Build from source

Requirements:

- [Go 1.22.3 or newer](https://golang.org/dl/)

### For development

_**Note:** These steps [will not embed proper version information](https://github.com/golang/go/issues/29228). For that, please follow the instructions in the next section._

```bash
$ git clone "https://github.com/khulnasoft/kengine.git"
$ cd kengine/cmd/kengine/
$ go build
```

When you run Kengine, it may try to bind to low ports unless otherwise specified in your config. If your OS requires elevated privileges for this, you will need to give your new binary permission to do so. On Linux, this can be done easily with: `sudo setcap cap_net_bind_service=+ep ./kengine`

If you prefer to use `go run` which only creates temporary binaries, you can still do this with the included `setcap.sh` like so:

```bash
$ go run -exec ./setcap.sh main.go
```

If you don't want to type your password for `setcap`, use `sudo visudo` to edit your sudoers file and allow your user account to run that command without a password, for example:

```
username ALL=(ALL:ALL) NOPASSWD: /usr/sbin/setcap
```

replacing `username` with your actual username. Please be careful and only do this if you know what you are doing! We are only qualified to document how to use Kengine, not Go tooling or your computer, and we are providing these instructions for convenience only; please learn how to use your own computer at your own risk and make any needful adjustments.

### With version information and/or plugins

Using [our builder tool, `xkengine`](https://github.com/khulnasoft/xkengine)...

```
$ xkengine build
```

...the following steps are automated:

1. Create a new folder: `mkdir kengine`
2. Change into it: `cd kengine`
3. Copy [Kengine's main.go](https://github.com/khulnasoft/kengine/blob/master/cmd/kengine/main.go) into the empty folder. Add imports for any custom plugins you want to add.
4. Initialize a Go module: `go mod init kengine`
5. (Optional) Pin Kengine version: `go get github.com/khulnasoft/kengine/v2@version` replacing `version` with a git tag, commit, or branch name.
6. (Optional) Add plugins by adding their import: `_ "import/path/here"`
7. Compile: `go build`

## Quick start

The [Kengine website](https://khulnasoft.com/docs/) has documentation that includes tutorials, quick-start guides, reference, and more.

**We recommend that all users -- regardless of experience level -- do our [Getting Started](https://khulnasoft.com/docs/getting-started) guide to become familiar with using Kengine.**

If you've only got a minute, [the website has several quick-start tutorials](https://khulnasoft.com/docs/quick-starts) to choose from! However, after finishing a quick-start tutorial, please read more documentation to understand how the software works. 🙂

## Overview

Kengine is most often used as an HTTPS server, but it is suitable for any long-running Go program. First and foremost, it is a platform to run Go applications. Kengine "apps" are just Go programs that are implemented as Kengine modules. Two apps -- `tls` and `http` -- ship standard with Kengine.

Kengine apps instantly benefit from [automated documentation](https://khulnasoft.com/docs/json/), graceful on-line [config changes via API](https://khulnasoft.com/docs/api), and unification with other Kengine apps.

Although [JSON](https://khulnasoft.com/docs/json/) is Kengine's native config language, Kengine can accept input from [config adapters](https://khulnasoft.com/docs/config-adapters) which can essentially convert any config format of your choice into JSON: Kenginefile, JSON 5, YAML, TOML, NGINX config, and more.

The primary way to configure Kengine is through [its API](https://khulnasoft.com/docs/api), but if you prefer config files, the [command-line interface](https://khulnasoft.com/docs/command-line) supports those too.

Kengine exposes an unprecedented level of control compared to any web server in existence. In Kengine, you are usually setting the actual values of the initialized types in memory that power everything from your HTTP handlers and TLS handshakes to your storage medium. Kengine is also ridiculously extensible, with a powerful plugin system that makes vast improvements over other web servers.

To wield the power of this design, you need to know how the config document is structured. Please see [our documentation site](https://khulnasoft.com/docs/) for details about [Kengine's config structure](https://khulnasoft.com/docs/json/).

Nearly all of Kengine's configuration is contained in a single config document, rather than being scattered across CLI flags and env variables and a configuration file as with other web servers. This makes managing your server config more straightforward and reduces hidden variables/factors.

## Full documentation

Our website has complete documentation:

**https://khulnasoft.com/docs/**

The docs are also open source. You can contribute to them here: https://github.com/khulnasoft/kengine-website

## Getting help

- We advise companies using Kengine to secure a support contract through [Ardan Labs](https://www.ardanlabs.com/my/contact-us?dd=kengine) before help is needed.

- A [sponsorship](https://github.com/sponsors/mholt) goes a long way! We can offer private help to sponsors. If Kengine is benefitting your company, please consider a sponsorship. This not only helps fund full-time work to ensure the longevity of the project, it provides your company the resources, support, and discounts you need; along with being a great look for your company to your customers and potential customers!

- Individuals can exchange help for free on our community forum at https://kengine.community. Remember that people give help out of their spare time and good will. The best way to get help is to give it first!

Please use our [issue tracker](https://github.com/khulnasoft/kengine/issues) only for bug reports and feature requests, i.e. actionable development items (support questions will usually be referred to the forums).

## About

Matthew Holt began developing Kengine in 2014 while studying computer science at Brigham Young University. (The name "Kengine" was chosen because this software helps with the tedious, mundane tasks of serving the Web, and is also a single place for multiple things to be organized together.) It soon became the first web server to use HTTPS automatically and by default, and now has hundreds of contributors and has served trillions of HTTPS requests.

**The name "Kengine" is trademarked.** The name of the software is "Kengine", not "Kengine Server" or "KhulnaSoft". Please call it "Kengine" or, if you wish to clarify, "the Kengine web server". Kengine is a registered trademark of Stack Holdings GmbH.

- _Project on Twitter: [@khulnasoft](https://twitter.com/khulnasoft)_
- _Author on Twitter: [@mholt6](https://twitter.com/mholt6)_

Kengine is a project of [ZeroSSL](https://zerossl.com), a Stack Holdings company.

Debian package repository hosting is graciously provided by [Cloudsmith](https://cloudsmith.com). Cloudsmith is the only fully hosted, cloud-native, universal package management solution, that enables your organization to create, store and share packages in any format, to any place, with total confidence.
