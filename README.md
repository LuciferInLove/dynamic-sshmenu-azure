[![Go Report Card](https://goreportcard.com/badge/github.com/LuciferInLove/dynamic-sshmenu-azure)](https://goreportcard.com/report/github.com/LuciferInLove/dynamic-sshmenu-azure)
[![License](https://img.shields.io/badge/license-MIT-red.svg)](./LICENSE.md)
![Build status](https://github.com/LuciferInLove/dynamic-sshmenu-azure/workflows/Build/badge.svg)

# dynamic-sshmenu-azure

Dynamically creates a menu containing a list of Microsoft Azure™ Virtual Machines selected using tags.

![dynamic-sshmenu-azure](https://user-images.githubusercontent.com/34190954/137604261-5074223c-4948-4333-b787-a1d00e72d2c9.gif)

## Overview

**dynamic-sshmenu-azure** generates sshmenu-style lists to connect to Azure™ virtual machines. It searches virtual machines by tags that you can define as arguments. **dynamic-sshmenu-azure** executes `ssh __ip_address__` after choosing a menu item.

## Preparations for using

First of all, you should setup authentication to interact with Azure™:
* [authentication methods](https://docs.microsoft.com/en-us/azure/developer/go/azure-sdk-authorization) **(NOTE: only file-based authentication works for now)**

If you are using bastion server, you can set it as proxy in ssh config as follows:

```
Host 172.31.*.*
  ProxyCommand ssh -W %h:%p 203.0.113.25
  ForwardAgent=yes
```

`172.31.*.*` - your virtual machines private addresses range, `203.0.113.25` - bastion server public ip.
Or you can set ProxyCommand using flag -s as follows:

```shell
dynamic-sshmenu-azure -s "-o ProxyCommand=ssh -W %h:%p 203.0.113.25"
```

[Use ssh agent forwarding](https://developer.github.com/v3/guides/using-ssh-agent-forwarding/) to prevent keeping your private ssh keys on bastion servers.

## Usage

You can see the **dynamic-sshmenu-azure** help by running it with `-h` argument.

### Command Line Options

	--tags value,           -t value    instance tags in "key1:value1;key2:value2" format. If undefined, full list will be shown
    --resource-group value, -g value    azure resource group name. If undefined, resource groups list will be shown. Environment variables: [$AZURE_DEFAULTS_GROUP, $AZURE_BASE_GROUP_NAME]
    --location value,       -l value    azure resource groups location (region). If undefined, full resource groups list will be shown. Environment variables: [$AZURE_DEFAULTS_LOCATION]
    --public-ip,            -p          use public ip instead of private. If vm doesn't have public ip, it will be skipped from the list (default: false)
    --ssh-username value,   -u value    ssh username. If undefined, the current user will be used
    --ssh-options value,    -s value    ssh additional parameters. You can specify, e.g., ProxyCommand, etc. Please, don't use additional quotes here.
    --help,                 -h          show help (default: false)
    --version,              -v          print the version (default: false)

## Windows limitations

The application doesn't work in [mingw](http://www.mingw.org/) or similar terminals. You can use default cmd.exe, [windows terminal](https://github.com/microsoft/terminal) or run linux version of **dynamic-sshmenu-azure** in [wsl](https://docs.microsoft.com/en/windows/wsl/install-win10). Windows doesn't provide ssh connections ability by default. You must have `ssh.exe` installed in any of [PATH](https://docs.microsoft.com/en-us/windows/win32/shell/user-environment-variables) directories. For example, you can install [GitBash](https://gitforwindows.org/).
