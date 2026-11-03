// Tests for the inline script of game.html, run with Node's built-in runner:
//   node --test internal/server/game_page.test.mjs
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import fs from 'node:fs';

const html = fs.readFileSync(new URL('./game.html', import.meta.url), 'utf8');
const scriptMatch = /<script>([\s\S]*?)<\/script>/.exec(html);
assert.ok(scriptMatch, 'game.html has no inline <script>');
const pageScript = scriptMatch[1];

const BAD_COUNT_REASON = 'bad value received for number of players, please try again with a number from 2 to 22';
const BLANK_NAME_REASON = 'player name is blank';
const SAVE_FAILED_REASON = "couldn't record the win, please try again";

class FakeElement {
  constructor(hidden = false) {
    this.hidden = hidden;
    this.value = '';
    this.onclick = null;
    this.listeners = {};
    this.text = '';
  }

  get innerText() {
    return this.text;
  }

  set innerText(v) {
    this.text = String(v);
  }

  get textContent() {
    return this.text;
  }

  set textContent(v) {
    this.text = String(v);
  }

  addEventListener(type, fn) {
    (this.listeners[type] ??= []).push(fn);
  }

  click() {
    const event = { type: 'click', target: this, preventDefault() {} };
    if (typeof this.onclick === 'function') {
      this.onclick(event);
    }
    for (const fn of this.listeners.click ?? []) {
      fn(event);
    }
  }

  reportValidity() {
    return true;
  }
}

function makeWebSocketClass() {
  return class FakeWebSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;
    static instances = [];

    constructor(url) {
      this.url = url;
      this.readyState = FakeWebSocket.CONNECTING;
      this.sent = [];
      this.onopen = null;
      this.onmessage = null;
      this.onclose = null;
      this.onerror = null;
      this.listeners = {};
      FakeWebSocket.instances.push(this);
    }

    addEventListener(type, fn) {
      (this.listeners[type] ??= []).push(fn);
    }

    send(data) {
      if (this.readyState !== FakeWebSocket.OPEN) {
        throw new Error('InvalidStateError: websocket is not open');
      }
      this.sent.push(String(data));
    }

    close() {
      this.readyState = FakeWebSocket.CLOSED;
    }

    fire(type, event) {
      if (typeof this['on' + type] === 'function') {
        this['on' + type](event);
      }
      for (const fn of this.listeners[type] ?? []) {
        fn(event);
      }
    }

    fireOpen() {
      this.readyState = FakeWebSocket.OPEN;
      this.fire('open', { type: 'open' });
    }

    fireMessage(data) {
      this.fire('message', { type: 'message', data });
    }

    fireClose(code) {
      this.readyState = FakeWebSocket.CLOSED;
      this.fire('close', { type: 'close', code, wasClean: code === 1000 });
    }
  };
}

// loadPage runs the page script in a fresh vm context against fake DOM and
// WebSocket objects and returns handles to them.
function loadPage() {
  const ids = [
    'game',
    'game-start',
    'player-count',
    'start-game',
    'declare-winner',
    'winner',
    'winner-button',
    'blind-value',
    'game-end',
  ];
  const hiddenAtStart = new Set(['declare-winner', 'game-end']);
  const els = Object.fromEntries(ids.map((id) => [id, new FakeElement(hiddenAtStart.has(id))]));

  const WebSocket = makeWebSocketClass();
  const location = { host: 'localhost:5000' };
  const sandbox = {
    document: {
      location,
      getElementById: (id) => {
        assert.ok(id in els, `page asked for unknown element #${id}`);
        return els[id];
      },
    },
    WebSocket,
    console,
  };
  sandbox.window = sandbox;
  sandbox.globalThis = sandbox;
  sandbox.window.location = location;

  vm.createContext(sandbox);
  vm.runInContext(pageScript, sandbox);

  return {
    els,
    WebSocket,
    sockets: WebSocket.instances,
    startWithCount(count) {
      els['player-count'].value = count;
      els['start-game'].click();
    },
    declareWinner(name) {
      els.winner.value = name;
      els['winner-button'].click();
    },
    blindText: () => els['blind-value'].innerText,
  };
}

// startGood starts a game with 3 players and delivers the first blind alert.
function startGood(page) {
  page.startWithCount('3');
  assert.equal(page.sockets.length, 1, 'start-game should open one websocket');
  page.sockets[0].fireOpen();
  page.sockets[0].fireMessage('Blind is now 100\n');
}

describe('game page', () => {
  it('opens a socket and sends the count when start-game is clicked', () => {
    const page = loadPage();

    page.startWithCount('3');
    assert.equal(page.sockets.length, 1);
    assert.equal(page.sockets[0].url, 'ws://localhost:5000/ws');
    page.sockets[0].fireOpen();

    assert.deepEqual(page.sockets[0].sent, ['3']);
  });

  it('good count then saved win ends the game only on close 1000', () => {
    const page = loadPage();

    page.startWithCount('3');
    page.sockets[0].fireOpen();
    assert.deepEqual(page.sockets[0].sent, ['3']);

    page.sockets[0].fireMessage('Blind is now 100\n');
    assert.match(page.blindText(), /Blind is now 100/);
    assert.equal(page.els['declare-winner'].hidden, false);

    page.declareWinner('Ruth');
    assert.deepEqual(page.sockets[0].sent, ['3', 'Ruth']);
    assert.equal(page.els['game-end'].hidden, true, 'game end shown before the server confirmed the win');

    page.sockets[0].fireClose(1000);
    assert.equal(page.els['game-end'].hidden, false);
    assert.equal(page.els.game.hidden, true);
  });

  it('bad count re-prompts on the same socket', () => {
    const page = loadPage();

    page.startWithCount('1');
    page.sockets[0].fireOpen();
    page.sockets[0].fireMessage(BAD_COUNT_REASON);

    assert.equal(page.els['game-start'].hidden, false, 'count form should be visible again');
    assert.equal(page.els['declare-winner'].hidden, true);
    assert.equal(page.blindText(), BAD_COUNT_REASON);
    assert.equal(page.els['game-end'].hidden, true);

    page.startWithCount('3');
    assert.equal(page.sockets.length, 1, 'a retry must reuse the open socket');
    assert.deepEqual(page.sockets[0].sent, ['1', '3']);
  });

  const winnerReasons = [
    ['a blank name', BLANK_NAME_REASON],
    ['a failed save', SAVE_FAILED_REASON],
  ];

  for (const [name, reason] of winnerReasons) {
    it(`winner reason keeps the winner form: ${name}`, () => {
      const page = loadPage();

      startGood(page);
      page.declareWinner('Ruth');
      page.sockets[0].fireMessage(reason);

      assert.equal(page.els['declare-winner'].hidden, false);
      assert.equal(page.els.game.hidden, false);
      assert.equal(page.els['game-end'].hidden, true);
      assert.equal(page.blindText(), reason);

      page.declareWinner('Chris');
      assert.equal(page.sockets.length, 1);
      assert.equal(page.sockets[0].sent.at(-1), 'Chris');
    });
  }

  it('an abnormal close is not game end', () => {
    const page = loadPage();

    startGood(page);
    page.declareWinner('Ruth');
    page.sockets[0].fireClose(1006);

    assert.equal(page.els['game-end'].hidden, true);
    assert.equal(page.blindText(), 'Connection closed');
  });

  it('an empty count is still sent so the server can answer with a reason', () => {
    const page = loadPage();

    page.startWithCount('');
    assert.equal(page.sockets.length, 1);
    page.sockets[0].fireOpen();

    assert.deepEqual(page.sockets[0].sent, ['']);
  });
});
