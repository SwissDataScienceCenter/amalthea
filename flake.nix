{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = nixpkgs.legacyPackages.${system};
      devshellPkgs = with pkgs; [
        jq
        go
        gopls
        gotools
        go-tools
        gore
        kind
        kubectl
        docker
        k9s
        python312
        poetry
        azure-cli
      ];
    in {
      formatter = pkgs.alejandra;
      devShells = {
        default = pkgs.mkShellNoCC {
          buildInputs = devshellPkgs;
        };
        eike = pkgs.mkShellNoCC {
          buildInputs = devshellPkgs;
          ## this is the naespace the operator runs in when doing "make run"
          RELEASE_NAMESPACE = "ci-renku-4088";
          shellHook = ''
            export FLAKE_ROOT="$(git rev-parse --show-toplevel)"
            export KUBECONFIG=~/.kube/config_azure
          '';
        };
      };
    });
}
