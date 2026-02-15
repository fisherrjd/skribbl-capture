{ pkgs ? import
    (fetchTarball {
      name = "jpetrucciani-2026-02-14";
      url = "https://github.com/jpetrucciani/nix/archive/3e631ab10e3ea8acdf1a89cd943f81572fafbf7f.tar.gz";
      sha256 = "10qb2605jr9dqg2vzdfc38zwh6giy81vhsrs6z25w8wpmy69mwg9";
    })
    { }
}:
let
  name = "skribbl-capture";

  tools = with pkgs; {
    cli = [
      jfmt
      nixup
    ];
    go = [
      go
      go-tools
      gopls
    ];
    scripts = pkgs.lib.attrsets.attrValues scripts;
  };

  scripts = with pkgs; { };
  paths = pkgs.lib.flatten [ (builtins.attrValues tools) ];
  env = pkgs.buildEnv {
    inherit name paths; buildInputs = paths;
  };
in
(env.overrideAttrs (_: {
  inherit name;
  NIXUP = "0.0.10";
})) // { inherit scripts; }
