import { readFile } from 'node:fs/promises';

export function parseArgs(argv) {
  const flags = {};
  const positionals = [];
  const errors = [];

  for (let index = 0; index < argv.length; index += 1) {
    const value = argv[index];

    if (!value.startsWith('--')) {
      positionals.push(value);
      continue;
    }

    const key = value.slice(2);
    if (!key) {
      errors.push('Invalid flag');
      continue;
    }

    if (key === 'help' || key === 'h') {
      flags.help = 'true';
      continue;
    }

    if (key.includes('=')) {
      const [flag, ...rest] = key.split('=');
      flags[flag] = rest.join('=');
      continue;
    }

    const nextValue = argv[index + 1];
    if (!nextValue || nextValue.startsWith('--')) {
      errors.push(`Missing value for --${key}`);
      continue;
    }

    flags[key] = nextValue;
    index += 1;
  }

  return { flags, positionals, errors };
}

export function getFlag(flags, names) {
  for (const name of names) {
    if (flags[name]) return flags[name];
  }
  return undefined;
}

export function requireFlag(flags, names, label) {
  const value = getFlag(flags, names);
  if (!value) {
    throw new Error(`Missing required flag: ${label}`);
  }
  return value;
}

export async function loadJsonPayload(flags) {
  const jsonValue = getFlag(flags, ['json', 'payload']);
  const fileValue = getFlag(flags, ['file']);

  if (jsonValue && fileValue) {
    throw new Error('Provide either --json or --file, not both');
  }

  if (jsonValue) {
    return JSON.parse(jsonValue);
  }

  if (fileValue) {
    const contents = await readFile(fileValue, 'utf8');
    return JSON.parse(contents);
  }

  throw new Error('Missing required JSON payload: use --json or --file');
}

export function parseJsonFlag(flags, name) {
  const value = flags[name];
  if (!value) return undefined;
  return JSON.parse(value);
}
