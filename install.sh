#!/bin/bash

OS=$(uname -s | tr '[:upper:]' '[:lower:]')

ARCH=$(uname -m)
case $ARCH in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *)
        echo -e "\033[31mError.\033[0m Processor architecture not supported: $ARCH"
        echo -e "Create a request with a \033[31mproblem\033[0m: https://github.com/Lifailon/usup/issues"
        exit 1
        ;;
esac

echo -e "Current system: \033[32m$OS\033[0m (architecture: \033[32m$ARCH\033[0m)"

case "$SHELL" in
    */bash) shellRc="$HOME/.bashrc" ;; # Debian/RHEL
    */zsh) shellRc="$HOME/.zshrc" ;;   # MacOS
    *)
        shellRc="$HOME/.profile"
        echo -e "Shell \033[34m$SHELL\033[0m not supported, \033[32mprofile\033[0m is used to add path to environment variables"
        ;;
esac

touch $shellRc
mkdir -p $HOME/.local/bin

grep -F 'export PATH=$PATH:$HOME/.local/bin' $shellRc > /dev/null || { 
    echo 'export PATH=$PATH:$HOME/.local/bin' >> $shellRc
    source "$shellRc" 2> /dev/null || . "$shellRc"
    echo -e "Added environment variable \033[34m$HOME/.local/bin\033[0m in \033[34m$shellRc\033[0m"
}

GITHUB_LATEST_VERSION=$(curl -L -sS -H 'Accept: application/json' https://github.com/Lifailon/usup/releases/latest | sed -e 's/.*"tag_name":"\([^"]*\)".*/\1/')
if [ -z "$GITHUB_LATEST_VERSION" ]; then
    echo -e "\033[31mError.\033[0m Unable to get the latest version from GitHub repository, check your internet connection."
    exit 1
else
    BIN_URL="https://github.com/Lifailon/usup/releases/download/$GITHUB_LATEST_VERSION/usup-$GITHUB_LATEST_VERSION-$OS-$ARCH"
    curl -L -sS "$BIN_URL" -o $HOME/.local/bin/usup
    chmod +x $HOME/.local/bin/usup
    if [ $OS = "darwin" ]; then
        xattr -d com.apple.quarantine $HOME/.local/bin/usup
    fi
    echo -e "✔  Installation completed \033[32msuccessfully\033[0m in \033[34m$HOME/.local/bin/usup\033[0m (version: $GITHUB_LATEST_VERSION)"
    echo -e "To launch the interface from anywhere, \033[32mre-login\033[0m to the current session or run the command: \033[32m. $shellRc\033[0m"
    exit 0
fi
