{
  pkgs,
  lib,
  config,
  ...
}:
{
  languages.go.enable = true;

  enterShell = ''
    just start
  '';
}
