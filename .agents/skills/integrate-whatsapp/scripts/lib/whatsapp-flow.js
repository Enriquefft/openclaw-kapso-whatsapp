const { getStringFlag, requireStringFlag } = require('./cli');

function requireScope(flags) {
  const phoneNumberId = getStringFlag(flags, 'phone-number-id');
  const businessAccountId = getStringFlag(flags, 'business-account-id');

  if (!phoneNumberId && !businessAccountId) {
    throw new Error('Provide --phone-number-id or --business-account-id');
  }

  if (phoneNumberId && businessAccountId) {
    throw new Error('Provide only one of --phone-number-id or --business-account-id');
  }

  return { phoneNumberId, businessAccountId };
}

function requireScopeId(flags) {
  const scope = requireScope(flags);
  return scope.phoneNumberId || scope.businessAccountId;
}

function buildScopeQuery(flags) {
  const scope = requireScope(flags);
  return {
    phone_number_id: scope.phoneNumberId,
    business_account_id: scope.businessAccountId
  };
}

function requireFlowId(flags) {
  return requireStringFlag(flags, 'flow-id');
}

module.exports = {
  requireScope,
  requireScopeId,
  buildScopeQuery,
  requireFlowId
};
