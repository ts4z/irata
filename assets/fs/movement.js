// A movement is what makes a clock go.  movement.js makes this poker clock go.

// TODO: Rationalize naming, this is schizophrenic about the naming convention.
// (Copilot isn't helping, it didn't follow the convention and I made it worse.)

"use strict";

function randN(n) { return Math.floor(Math.random() * n); }

const LISTENER_TIMEOUT = 55 * 1000 + randN(10000);

var next_level_sound = null;

async function sleep(ms) {
  await new Promise(resolve => setTimeout(resolve, ms));
}

function tournament_id() {
  var parts = window.location.pathname.split("/");
  return parseInt(parts[parts.length - 1]);
}

function playNextLevelSound(_ = undefined) {
  if (next_level_sound && last_model.State.SoundMuted !== true) {
      next_level_sound.play();
  }
}

// t is in milliseconds
function to_hmmss(t) {
  t += 999;

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
    return "0:00";
  }

  var seconds = parseInt(t / 1000);
  var h = parseInt(seconds / 3600);
  var m = parseInt((seconds - (h * 3600)) / 60);
  var s = seconds % 60;

  var hh = h;
  var mm = h > 0 && m < 10 ? "0"+m : m;
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

let next_level_complete_at = undefined, clock_controls_locked = true;

// Initialize last_model (the last model we loaded) with a fail-safe initial
// model value
var last_model = {
  // State is things that are written to the database.
  "Version": -1,
  "NextLevelSound": undefined,
  "State": {
    "CurrentLevelNumber": 0,
    "IsClockRunning": false,
    "TimeRemainingMillis": 59 * 60 * 1000,
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
  // Transients are things that are computed from State and Structure.
  "Transients": {
    "ProtocolVersion": undefined,
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
  console.log("footer download required");
  return true;
}

async function fetch_footers(want_footer_plugs_id) {
  const response = await fetch("/api/footerPlugs/" + want_footer_plugs_id, {});
  const footer_model = await response.json();

  if (want_footer_plugs_id != footer_model.FooterPlugsID) {
    console.log("footer plug id changed while fetching, bailing");
    return;
  }

  fetched_footer_plugs_id = want_footer_plugs_id;
  footers = footer_model.TextPlugs;
  shuffle_array(footers);
  next_footer();
}

let cached_fetch_footers_promise = function () {
  let cached_promise_fetches_id = -1;
  let cached_promise = undefined;

  // Return a promie that succeeds or takes a full minute (to prevent spamming).
  // On success, the underlying request should make the footers start.
  // On failure, we should put everything back into a state where we 
  // can restart this (notably, not updating the globals and resetting
  // the caching paramaters back to values likely to recreate the request).
  return function () {
    let want_footer_plugs_id = last_model.FooterPlugsID;
    if (want_footer_plugs_id == cached_promise_fetches_id) {
      return cached_promise;
    }

    cached_promise_fetches_id = want_footer_plugs_id;
    cached_promise = fetch_footers(want_footer_plugs_id).finally(() => {
      cached_promise_fetches_id = -1;
      cached_promise = undefined;
    })
    return Promise.any([cached_promise, sleep(60 * 1000)]);
  }
}();

const next_footer = (function () {
  var next_footer_offset = 99999;

  return function () {
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

function reset_footer_interval() {
  if (typeof footer_interval_id !== 'number') {
    throw new Error("footer_interval_id not a number?");
  }
  clearInterval(footer_interval_id);
  footer_interval_id = undefined;
  start_rotating_footers();
}

// Slideshow management
const slideshow_interval_ms = 30000;
let slideshow_enabled = false;
let slideshow_interval_id = undefined;
let current_slide_index = 0;

function count_slides() {
  return document.querySelectorAll('.slideshow-slide').length;
}

function show_slide(index) {
  const slides = document.querySelectorAll('.slideshow-slide');
  if (slides.length === 0) {
    return;
  }
  
  // Hide all slides
  slides.forEach(slide => slide.style.display = 'none');
  
  // Show the requested slide
  if (index >= 0 && index < slides.length) {
    slides[index].style.display = 'block';
  }
}

function next_slide() {
  const slideCount = count_slides();
  if (slideCount === 0) {
    return;
  }
  
  current_slide_index = (current_slide_index + 1) % slideCount;
  show_slide(current_slide_index);
}

function start_slideshow() {
  const slideCount = count_slides();
  if (slideCount === 0) {
    console.log("No slides available");
    return;
  }
  
  slideshow_enabled = true;
  current_slide_index = 0;
  
  // Show overlay and first slide
  const overlay = document.getElementById('slideshow-overlay');
  if (overlay) {
    overlay.style.display = 'block';
  }
  show_slide(current_slide_index);
  
  // Start rotation
  if (typeof slideshow_interval_id === 'undefined') {
    slideshow_interval_id = setInterval(next_slide, slideshow_interval_ms);
  }
}

function stop_slideshow() {
  slideshow_enabled = false;
  
  // Hide overlay
  const overlay = document.getElementById('slideshow-overlay');
  if (overlay) {
    overlay.style.display = 'none';
  }
  
  // Stop rotation
  if (typeof slideshow_interval_id === 'number') {
    clearInterval(slideshow_interval_id);
    slideshow_interval_id = undefined;
  }
}

async function listen_and_consume_model_changes(currentVersion, abortSignal) {
  const tid = tournament_id();
  if (!tid) {
    console.log("no tournament id");
    return Promise.reject("no tournament id");
  }
  const protocolVersion = last_model?.Transients?.ProtocolVersion ?? 0;

  const response = await fetch("/api/tournament-listen", {
    signal: abortSignal,
    method: "POST",
    mode: 'same-origin',
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ TournamentID: tid, Version: currentVersion, ProtocolVersion: protocolVersion }),
  });
  const model = await response.json();
  import_new_model_from_server(model);
  return "fetched new model";
}

function updateClockClassAndBannerFromLevel() {
  let cln = last_model.State.CurrentLevelNumber;
  let level = last_model.Structure.Levels[cln]

  if (level.IsBreak) {
    set_text("blinds", level.Description);
    set_class("clock-td", "clock-container clock-td-break");
  } else {
    set_text("blinds", level.Description);
    set_class("clock-td", "clock-container clock-td-running");
  }

  let level_banner = level.Banner;
  set_text("level", level_banner);
}

var bouncerID = undefined;

function maybeStopMuteBouncer() {
  if (bouncerID) {
    clearInterval(bouncerID);
    bouncerID = undefined;
  }
}

function maybeStartMuteBouncer() {
  if (bouncerID) {
    return;
  }
  bouncerID = setInterval(moveMuteBouncer, 1000/30);
}

// oh this is so dumb
const moveMuteBouncer = function () {
  // State for bouncer position and velocity (all in pixels)
  let x = window.innerWidth / 2;
  let y = window.innerHeight / 2;
  const pixelsToMove = 6;
  function randSign() { return randN(2)? 1 : -1 }
  let dx = 1+randN(pixelsToMove-2);
  let dy = pixelsToMove-dx;
  dx *= randSign();
  dy *= randSign();
  
  const el = document.getElementById("mute-bouncer");
  
  return function() {
    if (!el || el.style.display === "none") {
      return;
    }
    
    try {
      // Get viewport dimensions
      const viewportWidth = window.innerWidth;
      const viewportHeight = window.innerHeight;
      
      // Get element dimensions
      const rect = el.getBoundingClientRect();
      const elWidth = rect.width;
      const elHeight = rect.height;
      
      // Move by velocity
      x += dx;
      y += dy;
      
      // Check boundaries and bounce
      // Left/right edges
      if (x - elWidth/2 <= 0) {
        x = elWidth/2;
        dx = Math.abs(dx);  // Bounce right
      } else if (x + elWidth/2 >= viewportWidth) {
        x = viewportWidth - elWidth/2;
        dx = -Math.abs(dx);  // Bounce left
      }
      
      // Top/bottom edges
      if (y - elHeight/2 <= 0) {
        y = elHeight/2;
        dy = Math.abs(dy);  // Bounce down
      } else if (y + elHeight/2 >= viewportHeight) {
        y = viewportHeight - elHeight/2;
        dy = -Math.abs(dy);  // Bounce up
      }
      
      // Update element position
      el.style.left = x + "px";
      el.style.top = y + "px";
    } catch (e) {
      console.log("mute bouncer error: " + e);
      maybeStopMuteBouncer();
    }
  }
}();

// Server sent a whole new model.  Update all the fields.
function import_new_model_from_server(model) {
  console.log(`new model protocol=${model.Transients.ProtocolVersion} model.Version=${model.Version}`)

  if (model.State.Slideshow) {
    start_slideshow();
  } else {
    stop_slideshow();
  }

  let el = document.getElementById("mute-bouncer");
  if (el) {
    if (model.State.SoundMuted) {
      el.style.display = "block";
      maybeStartMuteBouncer();
    } else {
      el.style.display = "none";
      maybeStopMuteBouncer();
    }
  }

  if (model.NextLevelSoundID !== last_model.NextLevelSoundID) {
    if (model.Transients.NextLevelSoundPath) {
        next_level_sound = new Audio(model.Transients.NextLevelSoundPath);
    }
  } else {
    model.NextLevelSound = null;
  }

  if (last_model.Transients.ProtocolVersion &&
    model.Transients.ProtocolVersion != last_model.Transients.ProtocolVersion) {
    // new model changed the ProtocolVersion
    // this is tagged as an incompatible change
    window.location.reload();
  }

  last_model = model;

  update_time_fields();

  set_html("prize-pool", protect_html(model.State.PrizePool))
  set_text("current-players", model.State.CurrentPlayers)
  set_text("buyins", model.State.BuyIns)
  set_text("addons", model.State.AddOns)
  if (model.State.AddOns > 0) {
    show_els_by_ids(["addons-container"]);
  } else {
    hide_els_by_ids(["addons-container"]);
  }
  set_text("avg-chips", model.Transients.AverageChips)
  setNextDescription();
}

function is_clock_running() {
  return last_model.State.IsClockRunning;
}

function ms_until_next_break() {
  if (!is_clock_running()) {
    return undefined;
  }
  let amt = last_model.State.CurrentLevelEndsAt - Date.now();
  for (let i = last_model.State.CurrentLevelNumber + 1; i < last_model.Structure.Levels.length; i++) {
    let level = last_model.Structure.Levels[i];
    if (level.IsBreak) {
      return amt;
    }
    amt += level.DurationMinutes * 60 * 1000;
  }
  return undefined;
}

function next_non_break_level() {
  let cln = last_model.State.CurrentLevelNumber;
  let levels = last_model.Structure.Levels;
  for (let i = cln + 1; i < levels.length; i++) {
    if (!levels[i].IsBreak) {
      return levels[i];
    }
  }
  return null;
}

function setNextDescription() {
  let nnb = next_non_break_level();
  if (nnb !== null) {
      set_text("next-description", abridgeDescription(nnb.Description));
  }
}

// Try to abridge a level description to just the blinds/limits.
function abridgeDescription(desc) {
  let rxes = [ 
    /(?:BLINDS|LIMITS)\s+([0-9,k]+(?:-|—|\/|–)[0-9,k]+)/i ,
    /([0-9,k]+(?:-|—|\/|–)[0-9,k]+)/i,
  ];
  for (let rx of rxes) {
    let m = desc.match(rx);
    if (m) {
      return m[1];
    }
  }

  return desc;
}

function showPausedOverlay() {
  const show = !is_clock_running();
  const el = document.getElementById("paused-overlay");
  if (el) {
    el.style.display = show ? "block" : "none";
  }
}

function protect_html(s) {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;")
    .replace(/\r?\n/g, "<br>");
}

function set_text(id, value) {
  let el = document.getElementById(id)
  if (el !== null) {
    el.textContent = value
  } else {
    console.log("can't find element with id " + id)
  }
}

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

function show_els_by_ids(ids) {
  for (let id of ids) {
    const el = document.getElementById(id);
    if (el) {
      el.style.display = "block";
    }
  }
}

function hide_els_by_ids(ids) {
  for (let id of ids) {
    const el = document.getElementById(id);
    if (el) {
      el.style.display = "none";
    }
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
  let set = function(v) { set_text("next-break", v); }

  if (!is_clock_running()) {
    set("PAUSED");
    return;
  }

  let ms = ms_until_next_break();

  if (ms) {
    set(Math.floor(ms / (1000 * 60)) + " MIN");
  } else {
    set("N/A");
  }
}

async function maybe_clock_tick() {
  if (!is_clock_running()) {
    return Promise.reject("clock not running");
  }

  let time_until_next_tick = 1 + (millis_remaining_in_level() % 1000);
  // console.log(`ramaining until next tick: ${time_until_next_tick}`);
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
      let newLevel = last_model.Structure.Levels[last_model.State.CurrentLevelNumber];
      let nextDurationMinutes = newLevel.DurationMinutes;
      let oldMinutes = oldEndsAt.getMinutes();
      last_model.State.CurrentLevelEndsAt = new Date(oldEndsAt.setMinutes(oldMinutes + nextDurationMinutes)); // gross
      
      playNextLevelSound();
      if (newLevel.AutoPause) {
        // Trigger auto-pause.
        last_model.State.IsClockRunning = false;
      }
      setNextDescription();
    }

    update_time_fields();
  }
}

function update_time_fields() {
  update_break_clock();
  update_big_clock();
  updateClockClassAndBannerFromLevel();
  showPausedOverlay();
}

function update_big_clock() {
  var render = to_hmmss(millis_remaining_in_level());
  var clockElement = document.getElementById("clock");
  clockElement.innerHTML = render;

  // Add/remove clock-has-hours class for responsive sizing
  // Count colons to detect format: 1 colon = MM:SS (5 chars), 2 colons = H:MM:SS (7+ chars)
  var colonCount = (render.match(/:/g) || []).length;
  if (colonCount >= 2) {
    clockElement.classList.add("clock-has-hours");
  } else {
    clockElement.classList.remove("clock-has-hours");
  }
}

function footer_message(message_html) {
  reset_footer_interval();
  set_html("footer", message_html);
}

function installKeyboardHandlers(forWhom) {
  var isOp = (forWhom === 'operator');

  function toggle_clock_controls_lock(_) {
    clock_controls_locked = !clock_controls_locked;
  if (clock_controls_locked) {
      footer_message("level/clock controls re-locked");
    } else {
      footer_message("level/clock controls unlocked");
    }
  }

  function next_footer_key(_) {
    next_footer();
    clearInterval(footer_interval_id);
    footer_interval_id = setInterval(next_footer, next_footer_interval_ms);
  }

  function toggle_slideshow(_) {
    if (last_model === undefined) {
      console.log("last_model undefined")
    } else if (last_model.State.Slideshow) {
      send_modify('StopSlideshow');
    } else {
      send_modify('StartSlideshow');
    }
  }
  
  function send_modify(event, shift) {
    fetch('/api/keyboard-control', {
      method: 'POST',
      mode: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        "Event": event,
        "TournamentID": tournament_id(),
        "Shift": shift,
      })
    }).catch(error => console.log(`error in request for modify event ${event}: ${error}`));
  }

  function smwa(arg) {
    return function (shift) { send_modify(arg, shift); }
  }

  // if paused && clock unlocked, send modify with arg
  function ipcusmwa(arg) {
    return function (shift) {
      if (!clock_controls_locked) {
        send_modify(arg, shift);
      }
    }
  }

  function stopClock() {
    send_modify('StopClock');
  }

  function startClock() {
    send_modify('StartClock');
  }

  function toggle_pause(_) {
    if (last_model === undefined) {
      console.log("last_model undefined")
    } else if (is_clock_running()) {
      stopClock();
    } else {
      startClock();
    }
  }

  function toggle_mute(_) {
    if (!last_model && !last_model.State) {
      console.log("last_model or last_model.State undefined");
      return;
    }
    if (last_model.State.SoundMuted) {
      send_modify('UnmuteSound')
    } else {
      send_modify('MuteSound')
    }
  }

  var showing_help = false;

  function show_help_dialog(_) {
    const helpDialog = document.getElementById("help-dialog");
    if (helpDialog) {
      helpDialog.style.display = "block";
      showing_help = true;
    }
  }

  function hide_help_dialog(_) {
    const helpDialog = document.getElementById("help-dialog");
    if (helpDialog) {
      helpDialog.style.display = "none";
      showing_help = false;
    }
  }

  function handle_escape(_) {
    if (slideshow_enabled) {
      stop_slideshow();
    } else if (showing_help) {
      hide_help_dialog();
    } else {
      redirect('/');
    }
  }

  function redirect_to_edit(_) {
    redirect(window.location.pathname + "/edit");
  }

  const unauth_keycode_to_handler = {
    'Escape': handle_escape,
    'KeyF': next_footer_key,
    'KeyG': playNextLevelSound,
    'KeyM': toggle_mute,
    'KeyS': toggle_slideshow,
  };
  const operator_keycode_to_handler = {
    'ArrowDown': ipcusmwa('MinusMinute'),
    'ArrowLeft': ipcusmwa('PreviousLevel'),
    'ArrowRight': ipcusmwa('SkipLevel'),
    'ArrowUp': ipcusmwa('PlusMinute'),
    'Backspace': toggle_clock_controls_lock,
    'Comma': smwa('RemoveBuyIn'),
    'Delete': smwa('RemoveAddOn'),
    'End': smwa('RemoveBuyIn'),
    'Enter': toggle_pause,
    'Equal': smwa('AddBuyIn'),
    'Escape': handle_escape,
    'F1': show_help_dialog,
    'Home': smwa('AddBuyIn'),
    'Insert': smwa('AddAddOn'),
    'KeyE': redirect_to_edit,
    'KeyF': next_footer_key,
    'KeyG': playNextLevelSound,
    'KeyM': toggle_mute,
    'KeyR': smwa('Restart'),
    'KeyS': toggle_slideshow,
    'Minus': smwa('RemoveBuyIn'),
    'PageDown': smwa('RemovePlayer'),
    'PageUp': smwa('AddPlayer'),
    'Period': smwa('AddBuyIn'),
    'Slash': show_help_dialog,
    'Space': toggle_pause,
  }

  if (isOp) {
    document.addEventListener('keydown', (event) => {
      if (event.key === 'F1') {
        event.preventDefault();
      }
    }, false);
  }

  document.addEventListener('keyup', (event) => {
    if (event.ctrlKey) {
      console.log("not sending event, you're holding control");
      return;
    }
    let keycode_to_handler;
    if (isOp) {
      keycode_to_handler = operator_keycode_to_handler;
    } else {
      keycode_to_handler = unauth_keycode_to_handler;
    }
    let code = event.code;
    if (event.key === 'F1') {
      event.preventDefault();
    }
    var handler = keycode_to_handler[code];
    if (typeof handler !== 'undefined') {
      // console.log(`Key pressed ${event} ${code} => ${handler}`);
      handler(event.shiftKey);
    } else {
      // console.log(`drop key ${code}`)
    }
  }, false);

  if (isOp) {
    // mouse left/right = -/+ player
    // TODO: Make this optional, it makes debugging weird because
    // clicking in the browser re-syncs the model (becasue it jacks
    // up the player counter).
    document.addEventListener('click', (_) => {
      toggle_pause();
    }, false);

    document.addEventListener('contextmenu', (event) => {
      event.preventDefault();
      send_modify('RemovePlayer', event.shiftKey);
    }, false);
  }
}

// Listen to changes to the current version.
// This will cancel and make a new version if the version has
// changed since the previous call.  This way we use the same
// object across clock ticks.
const cached_change_listener = (() => {
  let listenerSentVersion, controller, cached_promise;

  function abort(why) {
    if (controller) {
      controller.abort(why);
    }
    controller = undefined;
    cached_promise = undefined;
  }

  function reset_cached_promise() {
    listenerSentVersion = last_model?.Version ?? 0;
    controller = new AbortController();

    let timeout = sleep(LISTENER_TIMEOUT).then(_ => abort("listener timed out normally"));
    let listener = 
          listen_and_consume_model_changes(listenerSentVersion, controller.signal)
          .catch((e) => {
            if (e.name === 'AbortError') {
              return Promise.reject("normal abort");
            } else {
              console.log(`cached_change_listener listen threw unexpected exception: ${e}`);
              return Promise.reject(e);
            }
          });

    cached_promise = Promise.any([timeout, listener])
      .catch((e) => {
        console.log(`cached_change_listener promise threw up: ${e}`);
      })
      .finally(() => { cached_promise = undefined; });
  }

  // prime the pump
  reset_cached_promise();

  function maybeResetCachedPromise() {
    if (!cached_promise) {
      reset_cached_promise();
      return;
    }

    let lmv = last_model?.Version ?? 0;
    // do we need an update?
    if (listenerSentVersion !== lmv) {
      abort("new version updated last_model");
      reset_cached_promise();
    }
  }

  return function () {
    maybeResetCachedPromise();
    return cached_promise;
  }
})();

// Wrapper around setTimeout(tick, ms), but prevents setting
// multiple tick timers.
//
// We continually reset the timer (that is, not an interval) because
// we want to get the timer close to the top of the second.  However,
// there are a couple places that schedule a tick, and a tick is self-
// scheduling.  So we quash one of them if we see a second one while
// the first one is outstanding.  (We don't use the interval id because
// this is a little clearer that it doesn't have a race condition, I think.)
const setTickTimer = function() {
  let counter = 1;

  return function(ms) {
    let c = ++counter;
    setTimeout(() => {
      if (counter !== c) {
        console.log(`skipping tick timer ${c}, counter is now ${counter}`);
        return;
      }
      tick();
    }, ms);
  }
}();

function tick() {
  let wait = [cached_change_listener()];
  if (is_clock_running()) {
    wait.push(maybe_clock_tick());
  }
  if (want_footers()) {
    wait.push(cached_fetch_footers_promise())
  }

  Promise.any(wait).catch((e) => {
    console.log(`tick threw up: ${e}`);
  }).finally(() => {
    // Schedule the start of the next tick, which will
    // mostly sleep until the top of the next second.  Do
    // this in 10ms in case we have a bug, we don't swamp
    // the browser.
    setTickTimer(10);
  });
}

start_rotating_footers();
tick();
