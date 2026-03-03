#!/usr/bin/env node
const { parseArgs, requireStringFlag } = require('./lib/cli');
const { platformRequest } = require('./lib/http');
const { run } = require('./lib/run');

run(async () => {
  const { flags } = parseArgs(process.argv.slice(2));
  const phoneNumberId = requireStringFlag(flags, 'phone-number-id');

  return platformRequest({
    method: 'GET',
    path: `/platform/v1/whatsapp/phone_numbers/${phoneNumberId}`
  });
});
