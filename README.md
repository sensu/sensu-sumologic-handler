[![Sensu Bonsai Asset](https://img.shields.io/badge/Bonsai-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/sensu/sensu-sumologic-handler)
![goreleaser](https://github.com/sensu/sensu-sumologic-handler/workflows/goreleaser/badge.svg)
[![Go Test](https://github.com/sensu/sensu-sumologic-handler/workflows/Go%20Test/badge.svg)](https://github.com/sensu/sensu-sumologic-handler/actions?query=workflow%3A%22Go+Test%22)
[![goreleaser](https://github.com/sensu/sensu-sumologic-handler/workflows/goreleaser/badge.svg)](https://github.com/sensu/sensu-sumologic-handler/actions?query=workflow%3Agoreleaser)


# Sensu Sumo Logic Handler

## Table of Contents
- [Overview](#overview)
- [Usage](#usage)
- [Environment variables](#environment-variables)
- [Annotations](#annotations)
- [Configuration](#configuration)
  - [Asset registration](#asset-registration)
  - [Handler definition](#handler-definition)
- [Installation from source](#installation-from-source)
- [Additional notes](#additional-notes)
- [Contributing](#contributing)

## Overview

The Sensu Sumo Logic Handler is a [Sensu Handler](https://docs.sensu.io/sensu-go/latest/reference/handlers/) that sends Sensu observability data (events and metrics) to a Sumo Logic [HTTP Logs and Metrics Source](https://help.sumologic.com/03Send-Data/Sources/02Sources-for-Hosted-Collectors/HTTP-Source).
The Sensu Sumo Logic handler automatically analyzes incoming observability data to determine if a Sumo log or a metric should be created (see `--always-send-log`, `--disable-send-log`, and `--disable-send-metric` to customize this behavior).

## Usage

```
Send Sensu observability data (events and metrics) to a hosted Sumo Logic HTTP Logs and Metrics Source.

Usage:
  sensu-sumologic-handler [flags]
  sensu-sumologic-handler [command]

Available Commands:
  help        Help about any command
  version     Print the version number of this plugin

Flags:
  -u, --url string                 Sumo Logic HTTP Logs and Metrics Source URL (Required)
  -a, --always-send-log            Always send event as log, even if metrics are present
      --disable-send-log           Disable send event as log
      --disable-send-metrics       Disable send event metrics
      --log-fields string          Custom Sumo Logic log fields (comma separated key=value pairs)
      --metric-dimensions string   Custom Sumo Logic metric dimensions (comma separated key=value pairs)
      --source-category string     Custom Sumo Logic source category (supports handler templates)
      --source-host string         Custom Sumo Logic source host (supports handler templates) (default "{{ .Entity.Name }}")
      --source-name string         Custom Sumo Logic source name (supports handler templates) (default "{{ .Check.Name }}")
  -n, --dry-run                    Dry-run, do not send data to Sumo Logic collector, report to stdout instead
  -v, --verbose                    Verbose output to stdout
  -h, --help                       help for sensu-sumologic-handler

Use "sensu-sumologic-handler [command] --help" for more information about a command.
```

## Environment variables

|Argument             |Environment Variable         |
|---------------------|-----------------------------|
|--url                |SUMOLOGIC_URL                |
|--source-name        |SUMOLOGIC_SOURCE_NAME        |
|--source-host        |SUMOLOGIC_SOURCE_HOST        |
|--source-category    |SUMOLOGIC_SOURCE_CATEGORY    |
|--metric-dimensions  |SUMOLOGIC_METRIC_DIMENSIONS  |
|--log-fields         |SUMOLOGIC_LOG_FIELDS         |

**Security Note:** Care should be taken to not expose the `--url` for this handler by specifying it on the command line or by directly setting the environment variable in the handler definition.
It is suggested to make use of [secrets management](https://docs.sensu.io/sensu-go/latest/operations/manage-secrets/secrets/) to surface it as an environment variable.
The example [handler definition](#handler-definition) below references the Sumo Logic URL as a secret.
Here is corresponding secret definition that make use of the built-in [env secrets provider](https://docs.sensu.io/sensu-go/latest/operations/manage-secrets/secrets-providers/#env-secrets-provider-example).

```yml
---
type: Secret
api_version: secrets/v1
metadata:
  name: sumologic_url
spec:
  provider: env
  id: SUMOLOGIC_URL
```

## Annotations

All of the command line arguments referenced in the help usage message can be overridden by check or entity annotations.
The annotation consists of the key formed by appending the "long" argument specification to the string `sensu.io/plugins/sumologic/config` (e.g. `sensu.io/plugins/sumologic/config/source-name`).

For example, having the following in an `agent.yml` file will create an entity annotation such that Sensu metrics sent to SumoLogic from this entity will include the additional metric-dimensions string `environment=production, entity=test` instead of the dimensions string defined with the handler command flag.

```yml
namespace: "default"
subscriptions:
  - linux
backend-url:
  - "ws://127.0.0.1:8081"
annotations:
  sensu.io/plugins/sumologic/config/metric-dimensions: "environment=production, entity=test"
```

## Configuration

### Asset registration

[Sensu Assets](https://docs.sensu.io/sensu-go/latest/reference/assets/) are the best way to make use of this plugin.
If you're not using an asset, please consider doing so!
If you're using sensuctl 5.13 with Sensu Backend 5.13 or later, you can use the following command to add the asset:

```
sensuctl asset add sensu/sensu-sumologic-handler
```

If you're using an earlier version of sensuctl, you can find the asset on the [Bonsai Asset Index](https://bonsai.sensu.io/assets/sensu/sensu-sumologic-handler).

### Handler definition

```yml
---
type: Handler
api_version: core/v2
metadata:
  name: sumologic
spec:
  command: >-
    sensu-sumologic-handler
    --source-host "{{ .Entity.Name }}"
    --source-name "{{ .Check.Name }}"
  type: pipe
  runtime_assets:
  - sensu/sensu-sumologic-handler
  secrets:
  - name: SUMOLOGIC_URL
    secret: sumologic_url
---
type: Secret
api_version: secrets/v1
metadata:
  name: sumologic_url
spec:
  provider: env
  id: SUMOLOGIC_HTTP_COLLECTOR_URL
```

#### Proxy Support

This handler supports the use of the environment variables `HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY` (or the lowercase versions thereof).
`HTTPS_PROXY` takes precedence over `HTTP_PROXY` for https requests.
The environment values may be either a complete URL or a `"host[:port]"` value (with no `http://` or `https://` prefix), in which case the "http" scheme is assumed.

## Installation from source

The preferred way of installing and deploying this plugin is to use it as an Asset.
If you would like to compile and install the plugin from source or contribute to it, download the latest version or create an executable script from this source.

From the local path of the sensu-sumologic-handler repository:

```
go build
```

## Additional notes

## Contributing

For more information about contributing to this plugin, see [Contributing](https://github.com/sensu/sensu-go/blob/master/CONTRIBUTING.md).
