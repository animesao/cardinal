# NixOS test configuration for dck
# This file demonstrates how to use dck in a NixOS configuration
#
# Usage in configuration.nix:
#
#   { config, pkgs, ... }:
#   {
#     imports = [
#       ./path/to/dck/flake.nix#nixosModules.dck
#     ];
#
#     services.dck = {
#       enable = true;
#       # dataDir = "/var/lib/dck";  # default
#       # user = "dck";              # default
#       # apiToken = "your-secret-token";
#       # apiPort = 2375;
#       # apiHost = "127.0.0.1";
#     };
#
#     # Open firewall for dck API (if needed externally)
#     networking.firewall.allowedTCPPorts = [ 2375 ];
#   }

{ config, lib, pkgs, ... }:

with lib;

let
  cfg = config.services.dck;
in
{
  # Example NixOS configuration using the dck module
  # This is a template - copy and customize for your use case

  services.dck = {
    enable = true;

    # Data directory for container state
    dataDir = "/var/lib/dck";

    # User to run dck as
    user = "dck";

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
    # Allow dck bridge traffic
    trustedInterfaces = [ "dck0" ];

    # Allow dck API (if needed externally)
    # allowedTCPPorts = [ 2375 ];
  };

  # Required packages for dck
  environment.systemPackages = with pkgs; [
    iproute2
    iptables
    util-linux
    procps
  ];
}
