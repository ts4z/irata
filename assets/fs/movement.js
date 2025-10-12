// A movement is what makes a clock go.  movement.js makes this poker clock go.

"use strict";

async function sleep(ms) {
  await new Promise(resolve => setTimeout(resolve, ms));
}

function tournament_id() {
  var parts = window.location.pathname.split("/");
  return parseInt(parts[parts.length - 1]);
}

function randN(n) { return Math.floor(Math.random() * n); }

// t is in milliseconds
function to_hmmss(t) {
  if (isNaN(t)) {
    console.log("can't clock " + t);
    return "♠♠:♠♠";
  }
  if (t === -1) {
    return "&nbsp;&nbsp;:&nbsp;&nbsp;";
  }
  if (typeof t === 'undefined') {
    return "♦♦:♦♦";
  }
  if (t < 1000) {
    return "00:00";
  }

  var seconds = parseInt(t / 1000);
  var h = parseInt(seconds / 3600);
  var m = parseInt((seconds - (h * 3600)) / 60);
  var s = seconds % 60;

  var hh = h;
  var mm = m >= 10 ? m : "0" + m;
  var ss = s >= 10 ? s : "0" + s;

  if (h === 0) {
    return mm + ":" + ss;
  } else {
    return hh + ":" + mm + ":" + ss;
  }
}

// https://dev.to/codebubb/how-to-shuffle-an-array-in-javascript-2ikj
function shuffle_array(a) {
  for (let i = a.length - 1; i > 0; i--) {
    const j = randN(i + 1);
    const temp = a[i];
    a[i] = a[j];
    a[j] = temp;
  }
}

const next_footer_interval_ms = 30000;

let next_level_complete_at = undefined, next_break_at = undefined, clock_controls_locked = true;

// Initialize last_model (the last model we loaded) with a fail-safe initial
// model value
var last_model = {
  // State is things that are written to the database.
  "State": {
    "CurrentLevelNumber": 0,
    "IsClockRunning": false,
    "TimeRemainingMillis": -1,
    "CurrentLevelEndsAt": undefined,
  },
  "Structure": {
    "Levels": [
      {
        "IsBreak": true,
        "Blinds": "LOADING...",
      },
      {
        "IsBreak": true,
        "Blinds": "FAKE BREAK LEVEL FOR INIT",
      },
    ]
  },
  // Transients are things that are computed from State and
  "Transients": {
    "NextBreakAt": undefined,
    "NextLevel": undefined,
    "EndsAt": undefined,
  }
}
var footers = [
  "ATOMIC BATTERIES TO POWER... TURBINES TO SPEED...",
  "RETICULATING SPLINES...",
  "CALIBRATING TIME AND SPACE...",
  "FLUXING CAPACITOR...",
  "CONGRATULATIONS, YOU AREN'T RUNNING EUNICE...",
  "TAPPING AQUARIUM...",
];
var fetched_footer_plugs_id = undefined;

function want_footers() {
  var want_id = last_model.FooterPlugsID
  if (!want_id) {
    return false;
  }
  if (fetched_footer_plugs_id && want_id === fetched_footer_plugs_id) {
    return false;
  }
  return true;
}

async function fetch_footers(abortSignal) {
  var want_footer_plugs_id = last_model.FooterPlugsID;
  const response = await fetch("/api/footerPlugs/" + want_footer_plugs_id, { signal: abortSignal });
  console.log("response " + response);
  const response_1 = response;
  const footer_model = await response_1.json();
  fetched_footer_plugs_id = want_footer_plugs_id;
  footers = footer_model.TextPlugs;
  shuffle_array(footers);
  next_footer();
  return "footers fetched";
}

const next_footer = (function () {
  var next_footer_offset = 99999;

  return function () {
    if (!clock_controls_locked) {
      return
    }

    next_footer_offset++;
    if (next_footer_offset > footers.length) {
      next_footer_offset = 0;
      shuffle_array(footers);
    }
    set_html("footer", footers[next_footer_offset % footers.length]);
  }
})()

let footer_interval_id = undefined;
function start_rotating_footers() {
  if (typeof footer_interval_id === 'undefined') {
    footer_interval_id = setInterval(next_footer, next_footer_interval_ms);
  }
}

function stop_rotating_footers() {
  if (typeof footer_interval_id !== 'undefined') {
    clearInterval(footer_interval_id);
    footer_interval_id = undefined;
  }
}

function listen_for_changes_once(abortSignal) {
  const tid = tournament_id();
  if (!tid) {
    console.log("no tournament id");
    return Promise.reject("no tournament id");
  }
  const version = last_model?.Version ?? 0;
  return fetch("/api/tournament-listen", {
    signal: abortSignal,
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tournament_id: tid, version: version })
  }).then(response => response.json())
    .then(model => import_new_model_from_server(model))
    .then(() => "fetched new model")
    .catch(e => {
      if (e.name === 'AbortError') {
        console.log(`fetch aborted ${e.stack}`);
        return Promise.reject("fetch aborted");
      } else {
        console.log(`fetch threw up: ${e}`);
        return Promise.reject(`fetch threw up: ${e}`);
      }
    });
}

function update_next_level_and_break_fields() {
  next_level_complete_at = last_model.Transients.EndsAt;
  next_break_at = last_model.Transients.NextBreakAt;

  var cln = last_model.State.CurrentLevelNumber;
  var level = last_model.Structure.Levels[cln]

  if (level.IsBreak) {
    set_html("blinds", level.Description);
    set_class("clock-td", "clock-break");
  } else {
    set_html("blinds", level.Description);
    set_class("clock-td", "clock");
  }

  let level_banner = level.Banner;
  set_html("level", level_banner);

  next_level_complete_at = new Date(last_model.State.CurrentLevelEndsAt)

  show_paused_overlay(!last_model.State.IsClockRunning);
}

// Server sent a whole new model.  Update all the fields.
function import_new_model_from_server(model) {
  console.log("import new model from server, version " + model.Version);
  last_model = model;

  update_time_fields();

  set_html("current-players", model.State.CurrentPlayers)
  set_html("buyins", model.State.BuyIns)
  // set rebuys
  // set addons
  set_html("avg-chips", model.Transients.AverageChips)
  if (model.Transients.NextLevel !== null) {
    set_html("next-level", model.Transients.NextLevel.Description)
  }
}

function show_paused_overlay(show) {
  const el = document.getElementById("paused-overlay");
  if (el) {
    el.style.display = show ? "block" : "none";
  }
}

// Helper.
function set_html(id, value) {
  let el = document.getElementById(id)
  if (el !== null) {
    el.innerHTML = value
  } else {
    console.log("can't find element with id " + id)
  }
}

function set_class(id, value) {
  var el = document.getElementById(id)
  if (el !== null) {
    el.className = value;
  } else {
    console.log("can't find element with id " + id)
  }
}

// Helper.
function redirect(where) {
  window.location.href = where;
}

function millis_remaining_in_level() {
  var ends_at = last_model?.State?.CurrentLevelEndsAt;
  if (ends_at) {
    return ends_at - Date.now();
  }

  var remaining = last_model?.State?.TimeRemainingMillis;
  if (typeof remaining !== 'undefined') {
    return remaining;
  }

  console.log("level_remaining: no ends_at or remaining, returning bogus 0");
  return 0;
}

function update_break_clock() {
  var td = document.getElementById("next-break");
  if (td === null) {
    console.log("update_break_clock: no next-break node to update");
    return;
  }

  if (!last_model.State.IsClockRunning) {
    td.innerHTML = "PAUSED";
    return
  }

  if (!last_model.Transients.NextBreakAt) {
    td.innerHTML = "N/A";
  } else if (typeof last_model.Transients.NextBreakAt !== 'number') {
    console.log("update_break_clock: NextBreakAt is nonsense");
    td.innerHTML = "???";
  } else {
    var remaining = (next_break_at - Date.now());
    if (remaining < 0) {
      console.log("remaining is < 0; next_break_at = " + next_break_at);
      remaining = 0;        // can't happen?
    }

    var mins = Math.floor(remaining / (1000 * 60));
    td.innerHTML = mins + " MIN";
  }
}

async function maybe_clock_tick() {
  if (!last_model.State.IsClockRunning) {
    return Promise.reject("clock not running");
  }

  let time_until_next_tick = 5 + (millis_remaining_in_level() % 1000);
  console.log(`ramaining until next tick: ${time_until_next_tick}`);
  return new Promise(resolve => setTimeout(resolve, time_until_next_tick)).then(() => {
    advance_clock_from_wall_clock();
    update_time_fields();
    return "clock ticked";
  });
}

function advance_clock_from_wall_clock() {
  var rem = millis_remaining_in_level();
  if (typeof rem === 'undefined') {
    // paused, no math to do?
    return;
  }

  if (rem <= 0) {
    let oldEndsAt = new Date(last_model.State.CurrentLevelEndsAt)
    last_model.State.CurrentLevelNumber++

    // fudge model while we wait for update from server

    if (last_model.State.CurrentLevelNumber >= last_model.Structure.Levels.length) {
      // fudge model while we wait for update from server
      console.log("it's the end of the world as we know it");
      last_model.State.CurrentLevelNumber = last_model.Structure.Levels.length - 1;
      last_model.State.IsClockRunning = false;
    } else {
      let nextDurationMinutes = last_model.Structure.Levels[last_model.State.CurrentLevelNumber].DurationMinutes;
      let oldMinutes = oldEndsAt.getMinutes();
      last_model.State.CurrentLevelEndsAt = new Date(oldEndsAt.setMinutes(oldMinutes + nextDurationMinutes)); // gross
    }

    update_time_fields();
  }
}

function update_time_fields() {
  update_break_clock(last_model);
  update_big_clock();
  update_next_level_and_break_fields();
}

function update_big_clock() {
  if (typeof next_level_complete_at === 'undefined') {
    // this doesn't happen -- why?
    document.getElementById("clock").innerHTML = "??:??";
  }
  var render = to_hmmss(millis_remaining_in_level());
  document.getElementById("clock").innerHTML = render
}


function install_keyboard_handlers() {
  console.log("installing keyboard handlers");

  function toggle_clock_controls_lock() {
    clock_controls_locked = !clock_controls_locked;
    if (clock_controls_locked) {
      console.log("clock controls locked");
      set_html("footer", "level/clock controls re-locked");
      start_rotating_footers();
    } else {
      console.log("clock unlocked");
      stop_rotating_footers();
      set_html("footer", "<nobr>level/clock controls</nobr> <nobr>available when paused</nobr>");
    }
  }

  function next_footer_key() {
    next_footer();
    clearInterval(footer_interval_id);
    footer_interval_id = setInterval(next_footer, next_footer_interval_ms);
  }

  function send_modify(event) {
    fetch('/api/keyboard-control', {
      method: 'POST',
      mode: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        "Event": event,
        "TournamentID": tournament_id(),
      })
    }).catch(error => console.log(`error in request for modify event ${event}: ${error}`));
  }

  function smwa(arg) {
    return function () { send_modify(arg); }
  }

  // if paused && clock unlocked, send modify with arg
  function ipcusmwa(arg) {
    return function () {
      if (!clock_controls_locked && !last_model.State.IsClockRunning) {
        send_modify(arg);
      } else {
        console.log(`clock_controls_locked=${clock_controls_locked} and running=$(last_model.State.IsClockRunning}`)
      }
    }
  }

  function toggle_pause() {
    if (last_model === undefined) {
      console.log("last_model undefined")
    } else if (last_model.State.IsClockRunning) {
      send_modify('StopClock')
    } else {
      send_modify('StartClock')
    }
  }

  var showing_help = false;

  function show_help_dialog() {
    const helpDialog = document.getElementById("help-dialog");
    if (helpDialog) {
      helpDialog.style.display = "block";
      showing_help = true;
    }
  }

  function hide_help_dialog() {
    const helpDialog = document.getElementById("help-dialog");
    if (helpDialog) {
      helpDialog.style.display = "none";
      showing_help = false;
    }
  }

  function handle_escape() {
    if (showing_help) {
      hide_help_dialog();
    } else {
      redirect('/');
    }
  }

  function redirect_to_edit() {
    redirect(window.location.pathname + "/edit");
  }

  var keycode_to_handler = {
    'Space': toggle_pause,
    'ArrowLeft': ipcusmwa('PreviousLevel'),
    'ArrowRight': ipcusmwa('SkipLevel'),
    'ArrowDown': ipcusmwa('MinusMinute'),
    'ArrowUp': ipcusmwa('PlusMinute'),
    'PageUp': smwa('AddPlayer'),
    'PageDown': smwa('RemovePlayer'),
    'Enter': toggle_pause,
    'Home': smwa('AddBuyIn'),
    'End': smwa('RemoveBuyIn'),
    'Equal': smwa('AddBuyIn'),
    'Minus': smwa('RemoveBuyIn'),
    'Comma': smwa('RemoveBuyIn'),
    'Period': smwa('AddBuyIn'),
    'KeyE': redirect_to_edit,
    'KeyF': next_footer_key,
    'Backspace': toggle_clock_controls_lock,
    'Escape': handle_escape,
    'Slash': show_help_dialog,
    'F1': show_help_dialog,
  }

  document.addEventListener('keydown', (event) => {
    if (event.key === 'F1') {
      event.preventDefault();
    }
  }, false);

  document.addEventListener('keyup', (event) => {
    var code = event.code;
    if (event.key === 'F1') {
      event.preventDefault();
    }
    var handler = keycode_to_handler[code];
    if (typeof handler !== 'undefined') {
      // console.log(`Key pressed ${event} ${code} => ${handler}`);
      handler();
    } else {
      console.log(`drop key ${code}`)
    }
  }, false);
}

// Listen to changes to the current version.
// This will cancel and make a new version if the version has
// changed since the previous call.  This way we use the same
// object across clock ticks.
const cached_change_listener = function () {
  let version = -1, controller = new AbortController(), cached_promise;
  return async function () {
    if (version != last_model.Version) {
      controller.abort("new version found");
      controller = new AbortController();
      version = last_model.Version;
      cached_promise = listen_for_changes_once(controller.signal);
    }
    return cached_promise;
  }
}();

async function tick() {
  const controller = new AbortController();
  const abortSignal = controller.signal;
  let wait = [cached_change_listener(), sleep(30*3600)];
  if (last_model.State.IsClockRunning) {
    wait.push(maybe_clock_tick());
  }
  if (want_footers()) {
    wait.push(fetch_footers(abortSignal));
  }

  Promise.any(wait).then((result) => {
    console.log(`awaited! result=${result}`);
  }).catch((e) => {
    console.log(`tick threw up: ${e}`);
  }).finally(() => {
    controller.abort("");
    setTimeout(tick, 50);
  });
}

tick();
