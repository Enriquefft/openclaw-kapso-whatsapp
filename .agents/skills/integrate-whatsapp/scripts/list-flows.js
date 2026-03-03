#!/usr/bin/env node
const { parseArgs, getStringFlag, getNumberFlag } = require('./lib/cli');
const { platformRequest } = require('./lib/http');
const { run } = require('./lib/run');

run(async () => {
  const { flags } = parseArgs(process.argv.slice(2));
  const phoneNumberId = getStringFlag(flags, 'phone-number-id') || getStringFlag(flags, 'phone_number_id');

  const query = {
    status: getStringFlag(flags, 'status'),
    business_account_id: getStringFlag(flags, 'business-account-id'),
    phone_number_id: phoneNumberId,
    name_contains: getStringFlag(flags, 'name-contains'),
    created_after: getStringFlag(flags, 'created-after'),
    created_before: getStringFlag(flags, 'created-before'),
    page: getNumberFlag(flags, 'page'),
    per_page: getNumberFlag(flags, 'per-page')
  };

  return platformRequest({
    method: 'GET',
    path: '/platform/v1/whatsapp/flows',
    query
  });
});
