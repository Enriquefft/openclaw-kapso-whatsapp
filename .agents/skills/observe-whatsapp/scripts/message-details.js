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
          usage: 'node scripts/message-details.js --message-id <wamid>',
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
    const messageId = flags['message-id'];
    if (!messageId || messageId === true) {
      throw new Error('Missing required flag --message-id');
    }

    const config = kapsoConfigFromEnv();
    const data = await kapsoRequest(
      config,
      `/platform/v1/whatsapp/messages/${encodeURIComponent(messageId)}`
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
