#!/usr/bin/env node
const { parseArgs, readFlagJson } = require('./lib/cli');
const { platformRequest } = require('./lib/http');
const { run } = require('./lib/run');
const { requireFlowId } = require('./lib/whatsapp-flow');

run(async () => {
  const { flags } = parseArgs(process.argv.slice(2));
  const flowId = requireFlowId(flags);
  const flowJson = await readFlagJson(flags, 'json', 'json-file');

  if (!flowJson) {
    throw new Error('Missing --json or --json-file');
  }

  return platformRequest({
    method: 'POST',
    path: `/platform/v1/whatsapp/flows/${flowId}/versions`,
    body: {
      flow_json: flowJson
    }
  });
});
