{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    devshell-tools.url = "github:eikek/devshell-tools";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix = {
      url = "github:tweag/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    devshell-tools,
    gomod2nix
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {
        inherit system;
        overlays = [ gomod2nix.overlays.default ];
      };
      goEnv = pkgs.mkGoEnv { pwd = ./.; };
      devshellPkgs = with pkgs; [
        jq
        go
        goEnv
        gopls
        gotools
        go-tools
        gore
        gomod2nix.packages.${system}.default
        kind
        kubectl
        docker
        k9s
        poetry
      ];
    in {
      formatter = pkgs.alejandra;
      packages.default = pkgs.buildGoApplication {
        name = "amalthea";
        src = pkgs.lib.cleanSource ./.;
        modules = ./gomod2nix.toml;
      };
      devShells = {
        default = pkgs.mkShellNoCC {
          buildInputs = devshellPkgs;
          shellHook = ''
            export FLAKE_ROOT="$(git rev-parse --show-toplevel)"
            export GOPATH="''${FLAKE_ROOT}/.go"
          '';
        };
      };
    });
}
