#!/usr/bin/env node
const { parseArgs, getStringFlag, getBooleanFlag, readFlagJson } = require('./lib/cli');
const { platformRequest } = require('./lib/http');
const { run } = require('./lib/run');

run(async () => {
  const { flags } = parseArgs(process.argv.slice(2));
  const phoneNumberId = getStringFlag(flags, 'phone-number-id') || getStringFlag(flags, 'phone_number_id');
  if (!phoneNumberId) {
    throw new Error('Missing required flag --phone-number-id');
  }

  const name = getStringFlag(flags, 'name');
  const publish = getBooleanFlag(flags, 'publish');
  const flowJson = await readFlagJson(flags, 'flow-json', 'flow-json-file');

  const body = {
    phone_number_id: phoneNumberId
  };

  if (name) body.name = name;
  if (flowJson) body.flow_json = flowJson;
  if (publish) body.publish = true;

  return platformRequest({
    method: 'POST',
    path: '/platform/v1/whatsapp/flows',
    body
  });
});
