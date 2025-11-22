# Development

<!--TOC-->

- [Development](#development)
  - [Intro](#intro)
  - [Workflow](#workflow)
  - [Troubleshooting](#troubleshooting)
    - [Dev Container](#dev-container)
      - [Linux](#linux)
        - [Podman](#podman)
      - [WSL2](#wsl2)

<!--TOC-->

## Intro

This repo is highly reliant on using devcontainers to keep the development environment the same over time and for all contributors.

All development "actions" are done via the provided Makefile

## Workflow

Here is a typical workflow for developing and testing this provider locally:

**1. Initial Setup**

Before you begin, you need to set up your environment. This is automated if you use the devcontainer.
If you are not using the devcontainer these are the steps for a fresh start:

*   **Install Go:** Make sure you have a working Go environment.
*   **Install Terraform/OpenTofu:** You'll need the Terraform or OpenTofu CLI to test the provider.
*   **Install `tfplugindocs`:** This tool is used to generate documentation. You can install it with:
    ```bash
    go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.19.4
    ```
*   **Configure Local Provider:** Create a `~/.terraformrc` file with the following content to tell Terraform to use your local build of the provider:
    ```
    provider_installation {
      filesystem_mirror {
        path    = "/dist"
        include = ["providers.localhost/dev/*"]
      }
      direct {
        exclude = ["providers.localhost/dev/*"]
      }
    }
    ```
*   **Add Go Binaries to PATH:** Make sure that the Go binary directory is in your `PATH`. You can do this by adding the following to your shell's startup file (e.g., `~/.bashrc`, `~/.zshrc`):
    ```bash
    export PATH=$PATH:$(go env GOPATH)/bin
    ```

**2. Building the Provider**

To compile the provider from the source code, run the following command from the root of the repository:

```bash
make build
```

This will place the compiled provider binary in the `/dist` directory, where your `~/.terraformrc` is configured to look for it.

**3. Testing**

*   **Running Automated Tests:** To run the project's built-in tests, use:
    ```bash
    make test
    ```
*   **Manual Testing with Examples:** The `examples/provider` directory is set up for manual testing.
    1.  Navigate to the `examples/provider` directory:
        ```bash
        cd examples/provider
        ```
    2.  Initialize Terraform. This will load your local provider:
        ```bash
        make init
        ```
    3.  Create a `testing.tf` file (or similar) in that directory to define some resources you want to test.
    4.  Run `tofu plan` to see what changes will be made, and `tofu apply` to apply them.

**4. Updating Documentation and Code**

After making changes to the provider's code or the underlying VyOS interface definitions, you need to regenerate the documentation and parts of the Go code. You can do this with the command we used earlier:

```bash
make ci-update
```

This command will:
*   Regenerate Go code from the VyOS interface definitions.
*   Update the provider schema.
*   Regenerate the documentation in the `docs/` directory.

After running this, you'll see a number of changed files. You can then commit them.

### Testing a Change (Example)

Let's say you've made a change to the provider and want to test it. For example, let's test a fix for setting the SSH service port.

1.  **Build the provider with your changes:**
    If you've modified the Go code, you need to rebuild the provider.
    ```bash
    make build
    ```

2.  **Navigate to the examples directory:**
    ```bash
    cd examples/provider
    ```

3.  **Create a test file:**
    Create a file named `testing.tf` and add the following configuration. This configuration will attempt to set the SSH port.

    ```terraform
    terraform {
      required_providers {
        vyos-rolling = {
          source = "providers.localhost/dev/vyos-rolling"
        }
      }
    }

    provider "vyos-rolling" {
      # Add your vyos connection details here
    }

    resource "vyos_service_ssh" "this" {
      port = "2222"
    }
    ```

4.  **Initialize and Apply:**
    Now, run `make init` to initialize Terraform with your local provider, then `tofu plan` and `tofu apply` to test the change.

    ```bash
    make init
    tofu plan
    tofu apply
    ```

    If the apply is successful, your change is working. You can then run `tofu destroy` to clean up the resources.

## Troubleshooting

### Dev Container

General References:

* [Sharing git credentials](https://code.visualstudio.com/remote/advancedcontainers/sharing-git-credentials)

General tips:

* Unless your user has `UID=1000`, and you are using rootless podman you will need to change the docker socket mount in `.devcontainer/devcontainer.json`

#### Linux

##### Podman

> Tip: podman-desktop is a nice and handy tool for a gui overview, for a tui client check out lazydocker

If you are using podman, make sure the docker socket compatibility is enabled: `sudo systemctl enable podman.socket`,
alternatively you can configure the devcontainer to mount the user socket, usually found at `/run/user/$(id -u)/podman/podman.sock`.

You can verify the socket access with curl, if it work it will show *"OK"*:
`curl -H "Content-Type: application/json" --unix-socket /var/run/docker.sock http://localhost/_ping`, if that does not work but the user socket works:
`curl -H "Content-Type: application/json" --unix-socket /run/user/$(id -u)/podman/podman.sock http://localhost/_ping`
you can try to communicate with the system socket using sudo.
If the system socket works as root, but not your user, you might need to add your user to the `podman` group (and log out and in again).

#### WSL2

Some workaround / issues must be handled to get dev containers to work well in WSL2.

1. Make sure you have a working SSH-Agent by updating the one in Windows
    * `Remove-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0`
    * `winget install openssh-beta`
    * [Reference](https://superuser.com/a/1722263)
2. Make sure GnuPG works:
    * In windows:
        * `winget install GnuPG.Gpg4win`
    * In WSL2:
        * `sudo apt-get install gpg gnupg gpg-agent`
        * `vim ~/.gnupg/gpg-agent.conf`
            ```
            default-cache-ttl 34560000
            max-cache-ttl 34560000
            pinentry-program "/mnt/c/Program Files (x86)/GnuPG/bin/pinentry-basic.exe"
            ```
        * `gpgconf --kill gpg-agent`
    * [Reference](https://www.39digits.com/signed-git-commits-on-wsl2-using-visual-studio-code)