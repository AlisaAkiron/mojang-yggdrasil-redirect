{
  description = "mojang-yggdrasil-redirect development environment";

  inputs = {
    nixpkgs.url = "https://channels.nixos.org/nixpkgs-unstable/nixexprs.tar.zst";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go_1_27 # compiler + toolchain
            gopls # language server
            golangci-lint # linter
            delve # debugger
            gotools # goimports etc.
          ];
        };
      }
    );
}
