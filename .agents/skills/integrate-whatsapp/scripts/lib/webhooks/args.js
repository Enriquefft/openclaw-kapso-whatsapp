function hasHelpFlag(argv) {
  return argv.includes('--help') || argv.includes('-h');
}

function parseFlags(argv) {
  const flags = {};
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (!arg.startsWith('--')) {
      continue;
    }
    const trimmed = arg.slice(2);
    const eqIndex = trimmed.indexOf('=');
    if (eqIndex >= 0) {
      const key = trimmed.slice(0, eqIndex);
      const value = trimmed.slice(eqIndex + 1);
      flags[key] = value;
      continue;
    }
    const next = argv[index + 1];
    if (!next || next.startsWith('--')) {
      flags[trimmed] = true;
      continue;
    }
    flags[trimmed] = next;
    index += 1;
  }
  return flags;
}

function requireFlag(flags, name) {
  const value = flags[name];
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error(`Missing required flag --${name}`);
  }
  return value;
}

function parseBoolean(value, name) {
  if (value === undefined) {
    return undefined;
  }
  if (value === true) {
    return true;
  }
  const normalized = String(value).toLowerCase();
  if (['true', '1', 'yes'].includes(normalized)) {
    return true;
  }
  if (['false', '0', 'no'].includes(normalized)) {
    return false;
  }
  throw new Error(`Invalid boolean for --${name}: ${value}`);
}

function parseNumber(value, name) {
  if (value === undefined || value === true) {
    return undefined;
  }
  const parsed = Number(value);
  if (Number.isNaN(parsed)) {
    throw new Error(`Invalid number for --${name}: ${value}`);
  }
  return parsed;
}

function parseJsonObject(value, name) {
  if (value === undefined || value === true) {
    return undefined;
  }
  try {
    const parsed = JSON.parse(value);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      throw new Error('Expected JSON object');
    }
    return parsed;
  } catch (error) {
    throw new Error(`Invalid JSON for --${name}: ${String(error)}`);
  }
}

function parseStringArray(value, name) {
  if (value === undefined || value === true) {
    return undefined;
  }
  const trimmed = String(value).trim();
  if (trimmed.startsWith('[')) {
    try {
      const parsed = JSON.parse(trimmed);
      if (!Array.isArray(parsed)) {
        throw new Error('Expected JSON array');
      }
      return parsed.map((entry) => String(entry));
    } catch (error) {
      throw new Error(`Invalid JSON array for --${name}: ${String(error)}`);
    }
  }
  return trimmed.split(',').map((entry) => entry.trim()).filter(Boolean);
}

module.exports = {
  hasHelpFlag,
  parseFlags,
  requireFlag,
  parseBoolean,
  parseNumber,
  parseJsonObject,
  parseStringArray
};
