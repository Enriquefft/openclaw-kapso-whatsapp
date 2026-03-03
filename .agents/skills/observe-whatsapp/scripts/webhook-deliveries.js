const { hasHelpFlag, parseFlags } = require('./lib/triage/args');
const { kapsoConfigFromEnv, kapsoRequest } = require('./lib/triage/kapso-api');

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
          usage:
            'node scripts/webhook-deliveries.js [--period <24h|7d|30d>] [--status <value>] [--event <value>] [--webhook-id <id>] [--errors-only true|false] [--page <n>] [--per-page <n>]',
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
    const params = new URLSearchParams();

    if (flags.period) params.set('period', flags.period);
    if (flags.status) params.set('status', flags.status);
    if (flags.event) params.set('event', flags.event);
    if (flags['webhook-id']) params.set('webhook_id', flags['webhook-id']);
    if (flags['errors-only'] !== undefined) params.set('errors_only', flags['errors-only']);
    if (flags.page) params.set('page', flags.page);
    if (flags['per-page']) params.set('per_page', flags['per-page']);

    const suffix = params.toString();
    const config = kapsoConfigFromEnv();
    const data = await kapsoRequest(
      config,
      `/platform/v1/webhook_deliveries${suffix ? `?${suffix}` : ''}`
    );

    console.log(JSON.stringify({ ok: true, data }, null, 2));
    return 0;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    console.error(JSON.stringify(err('Command failed', { message }), null, 2));
    return 1;
  }
}

main().then((code) => process.exit(code));
