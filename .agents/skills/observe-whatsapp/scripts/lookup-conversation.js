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
            'node scripts/lookup-conversation.js [--phone-number <e164>] [--phone-number-id <id>] [--conversation-id <uuid>] [--status <active|ended>] [--page <n>] [--per-page <n>]',
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
    const conversationId = flags['conversation-id'];
    const phoneNumber = flags['phone-number'];
    const phoneNumberId = flags['phone-number-id'] || flags.phone_number_id;

    const config = kapsoConfigFromEnv();

    if (conversationId && conversationId !== true) {
      const data = await kapsoRequest(
        config,
        `/platform/v1/whatsapp/conversations/${encodeURIComponent(conversationId)}`
      );
      console.log(JSON.stringify(ok(data), null, 2));
      return 0;
    }

    if ((!phoneNumber || phoneNumber === true) && !phoneNumberId) {
      throw new Error('Provide --conversation-id, --phone-number, or --phone-number-id');
    }

    const params = new URLSearchParams();
    if (phoneNumber && phoneNumber !== true) {
      params.set('phone_number', phoneNumber);
    }
    if (phoneNumberId) params.set('phone_number_id', phoneNumberId);
    if (flags.status) params.set('status', flags.status);
    if (flags.page) params.set('page', flags.page);
    if (flags['per-page']) params.set('per_page', flags['per-page']);

    const data = await kapsoRequest(
      config,
      `/platform/v1/whatsapp/conversations?${params.toString()}`
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
