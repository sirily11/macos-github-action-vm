packer {
  required_plugins {
    tart = {
      version = ">= 0.5.1"
      source  = "github.com/cirruslabs/tart"
    }
  }
}

source "tart-cli" "tart" {
  vm_base_name = "ghcr.io/cirruslabs/macos-tahoe-xcode:26.2"
  vm_name      = "runner"
  cpu_count    = 7
  memory_gb    = 7
  disk_size_gb = 120
  ssh_password = "admin"
  ssh_username = "admin"
  ssh_timeout  = "120s"
}

build {
  sources = ["source.tart-cli.tart"]
  // GitHub Runner
  provisioner "shell" {
    inline = [
      "source ~/.zprofile",
      "cd $HOME",
      "mkdir -p actions-runner && cd actions-runner",
      "RUNNER_TAG=$(curl -sL https://api.github.com/repos/actions/runner/releases/latest | jq -r '.tag_name')",
      "RUNNER_VERSION=$${RUNNER_TAG:1}",
      "curl -O -L https://github.com/actions/runner/releases/download/$RUNNER_TAG/actions-runner-osx-arm64-$RUNNER_VERSION.tar.gz",
      "tar xzf ./actions-runner-osx-arm64-$RUNNER_VERSION.tar.gz",
      "rm actions-runner-osx-arm64-$RUNNER_VERSION.tar.gz",
    ]
  }

  // Cert
  provisioner "shell" {
    inline = [
      "source ~/.zprofile",
      "set -euo pipefail",
      "tmp_dir=$(mktemp -d)",
      "install_cert(){ url=\"$1\"; name=\"$2\"; curl -fsSLo \"$tmp_dir/$name\" \"$url\"; if sudo /usr/bin/security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain \"$tmp_dir/$name\"; then return 0; fi; /usr/bin/security add-trusted-cert -d -r trustRoot -k \"$HOME/Library/Keychains/login.keychain-db\" \"$tmp_dir/$name\"; }",

      "install_cert https://developer.apple.com/certificationauthority/AppleWWDRCA.cer AppleWWDRCA.cer",
      "install_cert https://www.apple.com/certificateauthority/AppleWWDRCAG2.cer AppleWWDRCAG2.cer",
      "install_cert https://www.apple.com/certificateauthority/AppleWWDRCAG3.cer AppleWWDRCAG3.cer",
      "install_cert https://www.apple.com/certificateauthority/AppleWWDRCAG4.cer AppleWWDRCAG4.cer",
      "install_cert https://www.apple.com/certificateauthority/AppleWWDRCAG5.cer AppleWWDRCAG5.cer",
      "install_cert https://www.apple.com/certificateauthority/AppleWWDRCAG6.cer AppleWWDRCAG6.cer",

      "rm -rf \"$tmp_dir\"",
    ]
  }
}
