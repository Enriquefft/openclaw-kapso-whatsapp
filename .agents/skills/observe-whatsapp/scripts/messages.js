const { hasHelpFlag, parseFlags } = require('./lib/messages/args');
const { kapsoConfigFromEnv, kapsoRequest } = require('./lib/messages/kapso-api');

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
          usage:
            'node scripts/messages.js [--direction <inbound|outbound>] [--status <pending|sent|delivered|read|failed>] [--phone-number <e164>] [--conversation-id <uuid>] [--message-type <text|image|audio|video|document>] [--phone-number-id <id>] [--has-media true|false] [--page <n>] [--per-page <n>]',
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

    if (flags.direction) params.set('direction', flags.direction);
    if (flags.status) params.set('status', flags.status);
    if (flags['phone-number']) params.set('phone_number', flags['phone-number']);
    if (flags['conversation-id']) params.set('conversation_id', flags['conversation-id']);
    if (flags['message-type']) params.set('message_type', flags['message-type']);
    if (flags['phone-number-id']) params.set('phone_number_id', flags['phone-number-id']);
    if (flags.phone_number_id) params.set('phone_number_id', flags.phone_number_id);

    const hasMedia = parseBoolean(flags['has-media']);
    if (hasMedia !== undefined) params.set('has_media', String(hasMedia));

    if (flags.page) params.set('page', flags.page);
    if (flags['per-page']) params.set('per_page', flags['per-page']);

    const config = kapsoConfigFromEnv();
    const data = await kapsoRequest(
      config,
      `/platform/v1/whatsapp/messages${params.toString() ? `?${params.toString()}` : ''}`
    );

    console.log(JSON.stringify(ok(data), null, 2));
    return 0;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    console.error(JSON.stringify(err('Command failed', { message }), null, 2));
    return 1;
  }
}

function parseBoolean(value) {
  if (value === undefined) return undefined;
  if (value === true) return true;
  const normalized = String(value).toLowerCase();
  if (['true', '1', 'yes'].includes(normalized)) return true;
  if (['false', '0', 'no'].includes(normalized)) return false;
  throw new Error(`Invalid boolean for --has-media: ${value}`);
}

main().then((code) => process.exit(code));
