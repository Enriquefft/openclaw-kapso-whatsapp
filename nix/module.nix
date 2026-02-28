{ config, lib, pkgs, ... }:
let
  cfg = config.services.kapso-whatsapp;

  tomlFormat = pkgs.formats.toml { };

  configToml = tomlFormat.generate "kapso-whatsapp-config.toml" {
    delivery = {
      mode = cfg.delivery.mode;
      poll_interval = cfg.delivery.pollInterval;
      poll_fallback = cfg.delivery.pollFallback;
    };
    webhook = {
      addr = cfg.webhook.addr;
    };
    gateway = {
      url = cfg.gateway.url;
      session_key = cfg.gateway.sessionKey;
      sessions_json = cfg.gateway.sessionsJson;
    };
    state = {
      dir = cfg.state.dir;
    };
    security = {
      mode = cfg.security.mode;
      deny_message = cfg.security.denyMessage;
      rate_limit = cfg.security.rateLimit;
      rate_window = cfg.security.rateWindow;
      session_isolation = cfg.security.sessionIsolation;
      default_role = cfg.security.defaultRole;
    } // lib.optionalAttrs (cfg.security.roles != {}) {
      roles = cfg.security.roles;
    };
  };

  # Script that reads secret files and exports them as env vars before exec.
  loadSecrets = pkgs.writeShellScript "kapso-load-secrets" ''
    ${lib.optionalString (cfg.secrets.apiKeyFile != null) ''
      export KAPSO_API_KEY="$(cat ${cfg.secrets.apiKeyFile})"
    ''}
    ${lib.optionalString (cfg.secrets.phoneNumberIdFile != null) ''
      export KAPSO_PHONE_NUMBER_ID="$(cat ${cfg.secrets.phoneNumberIdFile})"
    ''}
    ${lib.optionalString (cfg.secrets.webhookVerifyTokenFile != null) ''
      export KAPSO_WEBHOOK_VERIFY_TOKEN="$(cat ${cfg.secrets.webhookVerifyTokenFile})"
    ''}
    ${lib.optionalString (cfg.secrets.webhookSecretFile != null) ''
      export KAPSO_WEBHOOK_SECRET="$(cat ${cfg.secrets.webhookSecretFile})"
    ''}
    ${lib.optionalString (cfg.secrets.gatewayTokenFile != null) ''
      export OPENCLAW_TOKEN="$(cat ${cfg.secrets.gatewayTokenFile})"
    ''}
    exec "$@"
  '';

  inherit (lib) mkEnableOption mkOption types mkIf;
in {
  options.services.kapso-whatsapp = {
    enable = mkEnableOption "Kapso WhatsApp bridge for OpenClaw";

    package = mkOption {
      type = types.package;
      description = "The kapso-whatsapp-poller package.";
    };

    cliPackage = mkOption {
      type = types.package;
      description = "The kapso-whatsapp-cli package.";
    };

    delivery = {
      mode = mkOption {
        type = types.enum [ "polling" "tailscale" "domain" ];
        default = "polling";
        description = "Message delivery mode.";
      };

      pollInterval = mkOption {
        type = types.int;
        default = 30;
        description = "Polling interval in seconds (minimum 5).";
      };

      pollFallback = mkOption {
        type = types.bool;
        default = false;
        description = "Run polling alongside webhook as a safety net.";
      };
    };

    webhook = {
      addr = mkOption {
        type = types.str;
        default = ":18790";
        description = "Webhook HTTP listen address.";
      };
    };

    gateway = {
      url = mkOption {
        type = types.str;
        default = "ws://127.0.0.1:18789";
        description = "OpenClaw gateway WebSocket URL.";
      };

      sessionKey = mkOption {
        type = types.str;
        default = "main";
        description = "OpenClaw session key.";
      };

      sessionsJson = mkOption {
        type = types.str;
        default = "${config.home.homeDirectory}/.openclaw/agents/main/sessions/sessions.json";
        description = "Path to the OpenClaw sessions JSON file.";
      };
    };

    state = {
      dir = mkOption {
        type = types.str;
        default = "${config.home.homeDirectory}/.config/kapso-whatsapp";
        description = "Directory for state files (last-poll timestamp, etc.).";
      };
    };

    security = {
      mode = mkOption {
        type = types.enum [ "allowlist" "open" ];
        default = "allowlist";
        description = "Security mode. 'allowlist' only allows numbers in roles. 'open' allows anyone.";
      };

      roles = mkOption {
        type = types.attrsOf (types.listOf types.str);
        default = { };
        example = {
          admin = [ "+1234567890" ];
          member = [ "+0987654321" "+1122334455" ];
        };
        description = "Role-grouped phone number allowlist. Each role maps to a list of phone numbers.";
      };

      denyMessage = mkOption {
        type = types.str;
        default = "Sorry, you are not authorized to use this service.";
        description = "Message sent to unauthorized senders.";
      };

      rateLimit = mkOption {
        type = types.int;
        default = 10;
        description = "Maximum messages per rate window per sender.";
      };

      rateWindow = mkOption {
        type = types.int;
        default = 60;
        description = "Rate limit window in seconds.";
      };

      sessionIsolation = mkOption {
        type = types.bool;
        default = true;
        description = "Give each sender their own OpenClaw session.";
      };

      defaultRole = mkOption {
        type = types.str;
        default = "member";
        description = "Role assigned to senders not in the roles map (used in 'open' mode).";
      };
    };

    secrets = {
      apiKeyFile = mkOption {
        type = types.nullOr types.str;
        default = null;
        description = "Path to file containing the Kapso API key.";
      };

      phoneNumberIdFile = mkOption {
        type = types.nullOr types.str;
        default = null;
        description = "Path to file containing the Kapso phone number ID.";
      };

      webhookVerifyTokenFile = mkOption {
        type = types.nullOr types.str;
        default = null;
        description = "Path to file containing the webhook verify token.";
      };

      webhookSecretFile = mkOption {
        type = types.nullOr types.str;
        default = null;
        description = "Path to file containing the webhook HMAC secret.";
      };

      gatewayTokenFile = mkOption {
        type = types.nullOr types.str;
        default = null;
        description = "Path to file containing the OpenClaw gateway token.";
      };
    };
  };

  config = mkIf cfg.enable {
    home.packages = [ cfg.cliPackage ];

    home.file.".config/kapso-whatsapp/config.toml".source = configToml;

    systemd.user.services.kapso-whatsapp-poller = {
      Unit = {
        Description = "Kapso WhatsApp Poller";
        After = [ "openclaw-gateway.service" ];
      };
      Service = {
        ExecStart = "${loadSecrets} ${cfg.package}/bin/kapso-whatsapp-poller";
        Environment = [ "KAPSO_CONFIG=%h/.config/kapso-whatsapp/config.toml" ];
        Restart = "on-failure";
      };
      Install.WantedBy = [ "default.target" ];
    };
  };
}
