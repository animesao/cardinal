# NixOS test configuration for cardinal
# This file demonstrates how to use cardinal in a NixOS configuration
#
# Usage in configuration.nix:
#
#   { config, pkgs, ... }:
#   {
#     imports = [
#       ./path/to/cardinal/flake.nix#nixosModules.cardinal
#     ];
#
#     services.cardinal = {
#       enable = true;
#       # dataDir = "/var/lib/cardinal";  # default
#       # user = "cardinal";              # default
#       # apiToken = "your-secret-token";
#       # apiPort = 2375;
#       # apiHost = "127.0.0.1";
#     };
#
#     # Open firewall for cardinal API (if needed externally)
#     networking.firewall.allowedTCPPorts = [ 2375 ];
#   }

{ config, lib, pkgs, ... }:

with lib;

let
  cfg = config.services.cardinal;
in
{
  # Example NixOS configuration using the cardinal module
  # This is a template - copy and customize for your use case

  services.cardinal = {
    enable = true;

    # Data directory for container state
    dataDir = "/var/lib/cardinal";

    # User to run cardinal as
    user = "cardinal";

    # API configuration (optional)
    # apiToken = "your-secure-token-here";
    # apiPort = 2375;
    # apiHost = "127.0.0.1";
  };

  # Required kernel modules for container namespaces
  boot.kernelModules = [
    "overlay"
    "veth"
    "br_netfilter"
  ];

  # Required sysctl settings
  boot.kernel.sysctl = {
    "net.ipv4.ip_forward" = 1;
    "net.bridge.bridge-nf-call-iptables" = 1;
    "net.bridge.bridge-nf-call-ip6tables" = 1;
  };

  # Firewall configuration
  networking.firewall = {
    # Allow cardinal bridge traffic
    trustedInterfaces = [ "cardinal0" ];

    # Allow cardinal API (if needed externally)
    # allowedTCPPorts = [ 2375 ];
  };

  # Required packages for cardinal
  environment.systemPackages = with pkgs; [
    iproute2
    iptables
    util-linux
    procps
  ];
}
