import { readFile } from 'node:fs/promises';
import path from 'node:path';

import { parseArgs, requireFlag } from './lib/args.mjs';
import { metaProxyRequest } from './lib/request.mjs';
import { err, ok, printResult } from './lib/output.mjs';

function usage() {
  return {
    usage: 'node scripts/upload-media.mjs --phone-number-id <PHONE_NUMBER_ID> --file <path> --mime-type <mime> [--filename <name>]',
    env: ['KAPSO_API_BASE_URL', 'KAPSO_API_KEY', 'META_GRAPH_VERSION (optional)'],
    notes: ['This returns a media id for send-time headers, not a template header_handle.'],
    hints: ['To discover phone_number_id (Meta phone number id), run: node scripts/list-platform-phone-numbers.mjs']
  };
}

async function main() {
  const { flags, errors } = parseArgs(process.argv.slice(2));
  if (flags.help) {
    return printResult(ok(usage()));
  }
  if (errors.length > 0) {
    return printResult(err('Invalid arguments', { errors, ...usage() }));
  }

  try {
    const phoneNumberId = requireFlag(flags, ['phone-number-id', 'phone_number_id'], 'phone-number-id');
    const filePath = requireFlag(flags, ['file'], 'file');
    const mimeType = requireFlag(flags, ['mime-type', 'mime_type'], 'mime-type');
    const filename = flags.filename || path.basename(filePath);

    const fileBuffer = await readFile(filePath);
    const blob = new Blob([fileBuffer], { type: mimeType });
    const formData = new FormData();

    formData.append('messaging_product', 'whatsapp');
    formData.append('type', mimeType);
    formData.append('file', blob, filename);

    const response = await metaProxyRequest({
      method: 'POST',
      path: `${phoneNumberId}/media`,
      body: formData
    });

    if (!response.ok) {
      return printResult(err('Meta proxy request failed', { response }));
    }

    return printResult(ok({ response }));
  } catch (error) {
    return printResult(err('Failed to upload media', { message: String(error?.message || error), ...usage() }));
  }
}

main().then((code) => process.exit(code));
