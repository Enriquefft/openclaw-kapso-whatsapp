const { readFile } = require('fs/promises');

function parseArgs(argv) {
  const flags = {};
  const positionals = [];

  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];

    if (!token.startsWith('--')) {
      positionals.push(token);
      continue;
    }

    const [rawKey, rawValue] = token.slice(2).split('=');
    if (rawValue !== undefined) {
      flags[rawKey] = rawValue;
      continue;
    }

    const next = argv[index + 1];
    if (next && !next.startsWith('--')) {
      flags[rawKey] = next;
      index += 1;
    } else {
      flags[rawKey] = true;
    }
  }

  return { flags, positionals };
}

function getFlag(flags, name) {
  return flags[name];
}

function getStringFlag(flags, name) {
  const value = getFlag(flags, name);
  if (value === undefined) return undefined;
  if (value === true) return 'true';
  return String(value);
}

function requireStringFlag(flags, name) {
  const value = getStringFlag(flags, name);
  if (!value) throw new Error(`Missing required flag --${name}`);
  return value;
}

function getBooleanFlag(flags, name) {
  const raw = getFlag(flags, name);
  if (raw === undefined) return undefined;
  if (raw === true) return true;
  const value = String(raw).toLowerCase();
  if (['true', '1', 'yes'].includes(value)) return true;
  if (['false', '0', 'no'].includes(value)) return false;
  throw new Error(`Invalid boolean for --${name}: ${raw}`);
}

function getNumberFlag(flags, name) {
  const raw = getStringFlag(flags, name);
  if (raw === undefined) return undefined;
  const value = Number(raw);
  if (Number.isNaN(value)) throw new Error(`Invalid number for --${name}: ${raw}`);
  return value;
}

async function readFlagText(flags, name, fileName) {
  const inlineValue = getStringFlag(flags, name);
  const fileValue = getStringFlag(flags, fileName);

  if (inlineValue && fileValue) {
    throw new Error(`Provide only one of --${name} or --${fileName}`);
  }

  if (fileValue) {
    return (await readFile(fileValue, 'utf8')).toString();
  }

  return inlineValue;
}

async function readFlagJson(flags, name, fileName) {
  const text = await readFlagText(flags, name, fileName);
  if (!text) return undefined;
  return JSON.parse(text);
}

module.exports = {
  parseArgs,
  getStringFlag,
  requireStringFlag,
  getBooleanFlag,
  getNumberFlag,
  readFlagText,
  readFlagJson
};
