## Setting Up docforge Locally for Contributing

### Prerequisites

Before you begin, ensure you have the following installed on your system:

- [Go](https://go.dev/dl/)
- [Git](https://git-scm.com/downloads)
- Make
   - **Windows**: See [Windows Setup Options](#windows-setup-options) below
   - **Mac**: Run `xcode-select --install`
   - **Linux**: Usually pre-installed

Verify the installation:

```sh
go version
git --version
make --version
```

#### Windows Setup Options

If you are on a Windows system, then you have several options for running docforge:

**Option 1: WSL (Windows Subsystem for Linux)**

WSL provides a native Linux environment on Windows, making it the easiest way to use docforge with full compatibility.

1. Install WSL:

   ```powershell
   # Run in PowerShell as Administrator
   wsl --install
   ```

   This command will install WSL with Ubuntu by default. Restart your computer when prompted.

1. Open WSL by searching for and launching "Ubuntu" in the Windows Start menu.

1. Install prerequisites in WSL:

   ```bash
   # Update package manager
   sudo apt update
   
   # Install Go
   wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
   echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
   source ~/.bashrc
   
   # Install Git and Make (usually pre-installed)
   sudo apt install git make
   ```

1. Continue with the [installation steps](#installation) below using the WSL terminal

**Option 2: Git Bash**

1. Install [Git for Windows](https://git-scm.com/downloads) which includes Git Bash:
1. Continue with the [installation steps](#installation) below using the GitBash terminal

> [!NOTE]
> If you are working on Windows, it is advisable to run all commands on Git, as the Makefile uses Unix-style bash scripts which may not work directly on Windows PowerShell or CMD.

### Installation

1. Clone the docforge repository locally via Git:

    ```bash
    git clone https://github.com/gardener/docforge.git
    cd docforge
    ```

1. Build Docforge (with Make):

    ```bash
    make build-local
    ```

1. Verify the installation:

    ```bash
    ./bin/docforge --help
    ```

### Configuration

1. Create a GitHub Token:
   1. Navigate to `GitHub Settings > Developer Settings > Personal Access Tokens`
   2. Generate a new token with a `repo` scope
   3. Save the token securely

1. Set the token as an environment variable:

    ```sh
    export GITHUB_TOKEN="your_token_here"
    ```

1. Create a configuration file (e.g., `docforge-config.yaml`) with the following content:

    ```yaml
    manifest: path/to/your/manifest.yaml
    destination: ./output
    hugo: true
    content-files-formats:
    - ".md"
    - ".html"
    - ".png"
    - ".jpg"
    - ".svg"
    markdown-enabled: true
    skip-link-validation: true
    ```

1. Run docforge:

    ```sh
    export DOCFORGE_CONFIG="docforge-config.yaml"
    ./bin/docforge --github-oauth-env-map "github.com=GITHUB_TOKEN"
    ```

## Testing Your Setup

1. Check the docforge version:

    ```bash
    ./bin/docforge version
    ```

1. Perform a dry run (tests without creating any files):

    ```sh
    ./bin/docforge -f manifest.yaml -d output --dry-run
    ```
