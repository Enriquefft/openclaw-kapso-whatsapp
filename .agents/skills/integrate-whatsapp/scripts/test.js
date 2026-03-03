const { hasHelpFlag, parseFlags, requireFlag } = require('./lib/webhooks/args');
const { kapsoConfigFromEnv, kapsoRequest } = require('./lib/webhooks/kapso-api');

function ok(data) {
  return { ok: true, data };
}

function err(message, details) {
  return { ok: false, error: { message, details } };
}

async function main() {
  const argv = process.argv.slice(2);
  if (hasHelpFlag(argv)) {
    console.log(
      JSON.stringify(
        {
          ok: true,
          usage: 'node scripts/test.js --webhook-id <id> [--event-type <value>]',
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
    const webhookId = requireFlag(flags, 'webhook-id');
    const eventType = flags['event-type'];

    const params = new URLSearchParams();
    if (eventType && eventType !== true) {
      params.set('event_type', eventType);
    }

    const config = kapsoConfigFromEnv();
    const data = await kapsoRequest(
      config,
      `/platform/v1/whatsapp/webhooks/${encodeURIComponent(webhookId)}/test${params.toString() ? `?${params.toString()}` : ''}`,
      { method: 'POST' }
    );

    console.log(JSON.stringify(ok(data), null, 2));
    return 0;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    console.error(JSON.stringify(err('Command failed', { message }), null, 2));
    return 1;
  }
}

main().then((code) => process.exit(code));
