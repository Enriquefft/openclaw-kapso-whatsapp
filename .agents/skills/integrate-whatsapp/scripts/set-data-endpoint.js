#!/usr/bin/env node
const { parseArgs, readFlagText } = require('./lib/cli');
const { platformRequest } = require('./lib/http');
const { run } = require('./lib/run');
const { requireFlowId } = require('./lib/whatsapp-flow');

run(async () => {
  const { flags } = parseArgs(process.argv.slice(2));
  const flowId = requireFlowId(flags);
  const code = await readFlagText(flags, 'code', 'code-file');
  if (!code) {
    throw new Error('Missing --code or --code-file');
  }

  return platformRequest({
    method: 'POST',
    path: `/platform/v1/whatsapp/flows/${flowId}/data_endpoint`,
    body: { code }
  });
});
