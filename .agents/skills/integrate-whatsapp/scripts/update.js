const { kapsoConfigFromEnv, kapsoRequest } = require('./lib/webhooks/kapso-api');
const { hasHelpFlag, parseFlags, requireFlag } = require('./lib/webhooks/args');
const { buildWebhookPayload } = require('./lib/webhooks/webhook');

function ok(data) {
  return { ok: true, data };
}

function err(message, details) {
  return { ok: false, error: { message, details } };
}

function resolveScope(flags) {
  const raw = flags.scope;
  if (!raw || raw === true) {
    return 'config';
  }
  const scope = String(raw);
  if (scope !== 'config' && scope !== 'project') {
    throw new Error(`Invalid --scope value: ${scope}`);
  }
  return scope;
}

async function main() {
  const argv = process.argv.slice(2);
  if (hasHelpFlag(argv)) {
    console.log(
      JSON.stringify(
        {
          ok: true,
          usage:
            'node scripts/update.js --webhook-id <id> [--phone-number-id <id>] [--scope config|project] [--url ...] [--events ...] [--kind <kapso|meta>] [--payload-version v1|v2] [--buffer-enabled true|false] [--buffer-window-seconds <n>] [--max-buffer-size <n>] [--inactivity-minutes <n>] [--headers <json>] [--active true|false]',
          env: ['KAPSO_API_BASE_URL', 'KAPSO_API_KEY']
        },
        null,
        2
      )
    );
    return 0;
  }

  try {
    const flags = parseFlags(argv);
    const scope = resolveScope(flags);
    const webhookId = requireFlag(flags, 'webhook-id');
    const payload = buildWebhookPayload(flags);
    if (Object.keys(payload).length === 0) {
      throw new Error('Provide at least one update field (e.g. --url, --events).');
    }

    const config = kapsoConfigFromEnv();
    let path = '';
    if (scope === 'project') {
      path = `/platform/v1/whatsapp/webhooks/${encodeURIComponent(webhookId)}`;
    } else {
      const phoneNumberId = requireFlag(flags, 'phone-number-id');
      path = `/platform/v1/whatsapp/phone_numbers/${encodeURIComponent(phoneNumberId)}/webhooks/${encodeURIComponent(webhookId)}`;
    }

    const data = await kapsoRequest(config, path, {
      method: 'PATCH',
      body: JSON.stringify({ whatsapp_webhook: payload })
    });

    console.log(JSON.stringify(ok(data), null, 2));
    return 0;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    console.error(JSON.stringify(err('Command failed', { message }), null, 2));
    return 1;
  }
}

main().then((code) => process.exit(code));
