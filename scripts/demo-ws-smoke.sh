#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
export PATH="$PWD/.tools/go/bin:$PWD/.tools/node/bin:$PATH"
port="${DEMO_PORT:-4274}"
temp_dir="$(mktemp -d)"
go build -o "$temp_dir/demo-server" ./server
GONAKO_BIN="$PWD/.tools/bin/gonako" "$temp_dir/demo-server" -addr "127.0.0.1:$port" >/dev/null 2>&1 &
server_pid=$!
trap 'kill "$server_pid" 2>/dev/null || true; rm -f "$temp_dir/demo-server"; rmdir "$temp_dir"' EXIT

for attempt in {1..40}; do
  if curl -fs "http://127.0.0.1:$port/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.25
done
curl -fsS "http://127.0.0.1:$port/healthz" | node -e '
let source=""; process.stdin.on("data", chunk => source += chunk); process.stdin.on("end", () => {
  const ready = JSON.parse(source);
  if (!ready.goReady || !ready.gonakoReady) process.exit(1);
});'
curl -fsS "http://127.0.0.1:$port/" >/dev/null
curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:$port/api/replays/room-unknown" | grep -qx 404

DEMO_PORT="$port" node <<'NODE'
const assert = require('node:assert/strict');
const net = require('node:net');
const crypto = require('node:crypto');

const port = Number(process.env.DEMO_PORT);
const host = `127.0.0.1:${port}`;
const origin = `http://${host}`;

class Peer {
  constructor(socket) {
    this.socket = socket;
    this.buffer = Buffer.alloc(0);
    this.messages = [];
    this.waiters = [];
    socket.on('data', chunk => { this.buffer = Buffer.concat([this.buffer, chunk]); this.parse(); });
  }
  parse() {
    while (this.buffer.length >= 2) {
      const opcode = this.buffer[0] & 15;
      let length = this.buffer[1] & 127;
      let offset = 2;
      if (length === 126) {
        if (this.buffer.length < 4) return;
        length = this.buffer.readUInt16BE(2);
        offset = 4;
      }
      if (this.buffer.length < offset + length) return;
      const payload = this.buffer.subarray(offset, offset + length);
      this.buffer = this.buffer.subarray(offset + length);
      const value = opcode === 1 ? JSON.parse(payload.toString()) : {opcode, payload};
      if (this.waiters.length) this.waiters.shift()(value);
      else this.messages.push(value);
    }
  }
  next() {
    if (this.messages.length) return Promise.resolve(this.messages.shift());
    return Promise.race([
      new Promise(resolve => this.waiters.push(resolve)),
      new Promise((_, reject) => setTimeout(() => reject(Error('WebSocket待機がタイムアウトしました')), 5000))
    ]);
  }
  send(value, masked = true) {
    const payload = Buffer.from(typeof value === 'string' ? value : JSON.stringify(value));
    const mask = Buffer.from([1, 2, 3, 4]);
    const head = Buffer.alloc(payload.length < 126 ? 2 : 4);
    head[0] = 0x81;
    head[1] = (masked ? 0x80 : 0) | (payload.length < 126 ? payload.length : 126);
    if (head.length === 4) head.writeUInt16BE(payload.length, 2);
    const body = Buffer.from(payload);
    if (masked) for (let i = 0; i < body.length; i++) body[i] ^= mask[i % 4];
    this.socket.write(Buffer.concat(masked ? [head, mask, body] : [head, body]));
  }
  close() { this.socket.destroy(); }
}

async function connect(playerId, suppliedOrigin = origin) {
  const socket = net.connect(port, '127.0.0.1');
  await new Promise(resolve => socket.once('connect', resolve));
  const key = crypto.randomBytes(16).toString('base64');
  const request = `GET /api/match?playerId=${playerId} HTTP/1.1\r\nHost: ${host}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: ${key}\r\nOrigin: ${suppliedOrigin}\r\n\r\n`;
  socket.write(request);
  let response = Buffer.alloc(0);
  while (!response.includes(Buffer.from('\r\n\r\n'))) {
    const chunk = await Promise.race([
      new Promise(resolve => socket.once('data', resolve)),
      new Promise((_, reject) => setTimeout(() => reject(Error('handshake待機がタイムアウトしました')), 5000))
    ]);
    response = Buffer.concat([response, chunk]);
  }
  const end = response.indexOf('\r\n\r\n') + 4;
  const status = response.subarray(0, end).toString().split('\r\n')[0];
  if (!status.includes('101')) { socket.destroy(); return {status}; }
  const peer = new Peer(socket);
  peer.buffer = response.subarray(end);
  peer.parse();
  return {status, peer};
}

(async () => {
  const rejected = await connect('origin-check', 'http://evil.example');
  assert.match(rejected.status, /403/);

  const first = (await connect('smoke-first')).peer;
  console.log('waiting');
  assert.equal((await first.next()).type, 'waiting');
  const second = (await connect('smoke-second')).peer;
  console.log('matched');
  const matchedFirst = await first.next();
  const matchedSecond = await second.next();
  assert.equal(matchedFirst.type, 'matched');
  assert.equal(matchedSecond.type, 'matched');
  assert.equal(matchedFirst.seat, 'dark');
  assert.equal(matchedSecond.seat, 'light');
  assert.deepEqual(matchedFirst.state, matchedSecond.state);
  const unfinished = await fetch(`${origin}/api/replays/${matchedFirst.roomId}`);
  assert.equal(unfinished.status, 404);
  assert.match((await unfinished.json()).error, /棋譜/);
  const missing = await fetch(`${origin}/api/replays/room-missing`);
  assert.equal(missing.status, 404);
  assert.match((await missing.json()).error, /棋譜/);

  first.send('{broken');
  console.log('invalid JSON');
  const badJSON = await first.next();
  assert.equal(badJSON.error.code, 'invalid_json');
  second.send({type:'pass', player:'light', expectedTurn:0});
  console.log('wrong seat');
  const wrongSeat = await second.next();
  assert.equal(wrongSeat.ok, false);
  assert.deepEqual(wrongSeat.state, matchedFirst.state);
  first.send({commandId:'smoke-bad-pass', type:'pass', player:'dark', expectedTurn:0});
  const badPass = await first.next();
  assert.equal(badPass.error.code, 'pass_not_allowed');
  assert.deepEqual(badPass.state, matchedFirst.state);

  const move = matchedFirst.state.legalMoves[0];
  first.send({commandId:'smoke-dark-1', type:'place', player:'dark', expectedTurn:0, row:move.row, col:move.col});
  console.log('dark move');
  const nextFirst = await first.next();
  const nextSecond = await second.next();
  assert.equal(nextFirst.type, 'state');
  assert.equal(nextSecond.type, 'state');
  assert.deepEqual(nextFirst.state, nextSecond.state);
  assert.equal(nextFirst.state.turnNumber, 1);

  first.send({commandId:'smoke-stale', type:'place', player:'dark', expectedTurn:0, row:move.row, col:move.col});
  console.log('stale move');
  const stale = await first.next();
  assert.equal(stale.error.code, 'stale_turn');
  assert.deepEqual(stale.state, nextFirst.state);

  const whiteMove = nextSecond.state.legalMoves[0];
  second.send({commandId:'smoke-light-1', type:'place', player:'light', expectedTurn:1, row:whiteMove.row, col:whiteMove.col});
  console.log('light move');
  const secondFirst = await first.next();
  const secondSecond = await second.next();
  assert.equal(secondFirst.state.turnNumber, 2);
  assert.deepEqual(secondFirst.state, secondSecond.state);

  if (process.env.DEMO_FINISH === '1') {
    let state = secondFirst.state;
    let safety = 0;
    while (state.phase !== 'finished' && safety++ < 130) {
      const actor = state.currentPlayer === 'dark' ? first : second;
      const command = {commandId:`smoke-finish-${state.turnNumber}`, type:'pass',
        player:state.currentPlayer, expectedTurn:state.turnNumber};
      if (state.legalMoves.length) {
        command.type = 'place';
        command.row = state.legalMoves[0].row;
        command.col = state.legalMoves[0].col;
      }
      actor.send(command);
      const one = await first.next();
      const two = await second.next();
      assert.equal(one.type, 'state');
      assert.equal(two.type, 'state');
      assert.deepEqual(one.state, two.state);
      state = one.state;
    }
    assert.equal(state.phase, 'finished');
    const response = await fetch(`${origin}/api/replays/${state.gameId}`);
    assert.equal(response.status, 200);
    const record = await response.json();
    assert.match(record.source, /ルール版宣言/);
    assert.match(record.source, /対局終了/);
    assert.match(record.source, new RegExp(`${state.gameId}.*対局開始`));
    assert.equal((record.source.match(/(黒|白)(着手|パス)/g) || []).length, state.turnNumber);
    assert.equal(record.frames[0].moveNumber, 0);
    assert.deepEqual(record.frames.at(-1).state, state);
    assert.equal(record.frames.length, state.turnNumber + 1);
    assert.equal(record.winner, state.winner);
    console.log(`demo replay smoke: PASS (${state.turnNumber}手)`);
  }

  if (process.env.DEMO_FINISH !== '1') {
    second.close();
    console.log('disconnect');
    assert.equal((await first.next()).type, 'opponentLeft');
    const returned = (await connect('smoke-second')).peer;
    console.log('reconnect');
    const current = await returned.next();
    assert.equal(current.seat, 'light');
    assert.deepEqual(current.state, secondFirst.state);
    assert.equal((await first.next()).type, 'opponentReturned');
    returned.close(); first.close();
  } else {
    second.close(); first.close();
  }

  const unmasked = (await connect('unmasked')).peer;
  console.log('unmasked');
  assert.equal((await unmasked.next()).type, 'waiting');
  unmasked.send('{}', false);
  const closed = await unmasked.next();
  assert.equal(closed.opcode, 8);
  assert.equal(closed.payload.readUInt16BE(), 1002);
  unmasked.close();
  console.log('demo WebSocket smoke: PASS');
})().catch(error => { console.error(error); process.exit(1); });
NODE
