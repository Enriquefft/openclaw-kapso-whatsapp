const { printOk, printError } = require('./output');
const { RequestError } = require('./http');

async function run(action) {
  try {
    const data = await action();
    printOk(data);
    return 0;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);

    if (error instanceof RequestError) {
      const blocked = error.status === 404;
      const details = {
        status: error.status,
        body: error.body,
        blocked
      };
      if (blocked) {
        details.hint = 'Endpoint not available in Meta proxy or Platform API yet.';
      }
      printError(message, details);
      return blocked ? 3 : 1;
    }

    printError(message, { blocked: false });
    return 1;
  }
}

module.exports = {
  run
};
