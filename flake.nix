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

      chartpress = pkgs.python3Packages.buildPythonApplication {
        pname = "chartpress";
        version = "2.3.0";
        pyproject = true;
        build-system = with pkgs.python3Packages; [ setuptools ];
        propagatedBuildInputs = with pkgs.python3Packages; [ docker ruamel-yaml ];
        src = pkgs.fetchFromGitHub {
          owner = "jupyterhub";
          repo = "chartpress";
          rev = "2.3.0";
          sha256 = "sha256-HBfXiz06nlScy2wbL2fFR5uopfxDNxLZpgoTQWBNn/U=";
        };
      };

      devshellPkgs = with pkgs; [
        jq
        yq-go
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
        chartpress
        poetry
        azure-cli
        kubernetes-helm
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
            export KUBECONFIG=~/.kube/config_kind
          '';
        };
      };
    });
}
