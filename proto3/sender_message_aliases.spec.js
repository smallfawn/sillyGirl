"use strict";

const assert = require("node:assert/strict");
const { sender } = require("./sillygirl");

async function main() {
  const expected = "sender-message-alias-fixture";
  sender.getMsg = async () => expected;

  assert.equal(await sender.getMsg(), expected);
  assert.equal(typeof sender.getMsgId, "function");
  assert.equal(typeof sender.setMsg, "function");
}

main().then(
  () => process.exit(0),
  (error) => {
    global.console.error(error);
    process.exit(1);
  },
);
