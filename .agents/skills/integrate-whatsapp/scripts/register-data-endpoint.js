#!/usr/bin/env node
const { parseArgs, getStringFlag } = require('./lib/cli');
const { platformRequest } = require('./lib/http');
const { run } = require('./lib/run');
const { requireFlowId } = require('./lib/whatsapp-flow');

run(async () => {
  const { flags } = parseArgs(process.argv.slice(2));
  const flowId = requireFlowId(flags);
  const phoneNumberId = getStringFlag(flags, 'phone-number-id') || getStringFlag(flags, 'phone_number_id');

  const body = {};
  if (phoneNumberId) {
    body.phone_number_id = phoneNumberId;
  }

  return platformRequest({
    method: 'POST',
    path: `/platform/v1/whatsapp/flows/${flowId}/data_endpoint/register`,
    body: Object.keys(body).length > 0 ? body : undefined
  });
});
