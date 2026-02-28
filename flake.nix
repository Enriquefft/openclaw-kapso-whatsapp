{
  description = "OpenClaw plugin: Kapso WhatsApp bridge";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };

        cli = pkgs.buildGoModule {
          pname = "kapso-whatsapp-cli";
          version = "0.1.0";
          src = ./.;
          subPackages = [ "cmd/kapso-whatsapp-cli" ];
          vendorHash = "sha256-0Qxw+MUYVgzgWB8vi3HBYtVXSq/btfh4ZfV/m1chNrA=";
          env.CGO_ENABLED = "0";
        };

        webhook = pkgs.buildGoModule {
          pname = "kapso-whatsapp-webhook";
          version = "0.1.0";
          src = ./.;
          subPackages = [ "cmd/kapso-whatsapp-webhook" ];
          vendorHash = "sha256-0Qxw+MUYVgzgWB8vi3HBYtVXSq/btfh4ZfV/m1chNrA=";
          env.CGO_ENABLED = "0";
        };
      in {
        packages = {
          inherit cli webhook;
          default = cli;
        };

        openclawPlugin = {
          name = "kapso-whatsapp";
          skills = [ ./skills/whatsapp ];
          packages = [ cli ];
          needs = {
            stateDirs = [ ".config/kapso-whatsapp" ];
            requiredEnv = [ "KAPSO_API_KEY" "KAPSO_PHONE_NUMBER_ID" ];
          };
        };
      }
    );
}
