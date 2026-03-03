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
            'node scripts/api-logs.js [--period <24h|7d|30d>] [--endpoint <value>] [--status-code <code>] [--errors-only true|false] [--page <n>] [--per-page <n>]',
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
    if (flags.endpoint) params.set('endpoint', flags.endpoint);
    if (flags['status-code']) params.set('status_code', flags['status-code']);
    if (flags['errors-only'] !== undefined) params.set('errors_only', flags['errors-only']);
    if (flags.page) params.set('page', flags.page);
    if (flags['per-page']) params.set('per_page', flags['per-page']);

    const suffix = params.toString();
    const config = kapsoConfigFromEnv();
    const data = await kapsoRequest(
      config,
      `/platform/v1/api_logs${suffix ? `?${suffix}` : ''}`
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
