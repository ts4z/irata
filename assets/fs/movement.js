// A movement is what makes a clock go.  movement.js makes this poker clock go.

"use strict";

function tournament_id() {
    var parts = window.location.pathname.split("/");
    return parts[parts.length - 1];
}

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
  var m = parseInt((seconds-(h*3600))/60);
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
const shuffle_array = array => {
  for (let i = array.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    const temp = array[i];
    array[i] = array[j];
    array[j] = temp;
  }
}

  const reload_model_ms = 15000;
  const clock_tick_ms = 250;
  const next_footer_interval_ms = 30000;

  let next_level_complete_at = undefined, next_break_at = undefined, clock_locked = true;
  
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
          "IsBreak":true,
          "Blinds":"LOADING...",
        },
        {
          "IsBreak":true,
          "Blinds":"FAKE BREAK LEVEL FOR INIT",
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
  var fetched_footer_plugs_id = NaN;

  async function maybe_fetch_footers(footer_plugs_id) {
    if (footer_plugs_id === fetched_footer_plugs_id) {
      return
    }
    fetch("/api/footerPlugs/" + footer_plugs_id)
      .then(response => {
        console.log("response " + response);
        return response;
      })
      .then(response => response.json())
      .then(model => {
        fetched_footer_plugs_id = footer_plugs_id;
        footers = model.TextPlugs
        shuffle_array(footers);
        next_footer();
      })
      .catch(error => console.log("error getting footers: ", error))
  }

  const next_footer = (function() {
    var next_footer_offset = 99999;

    return function() {
        if (!clock_locked) {
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
  
  async function load() {
    console.log("called: load()");
    fetch("/api/model/" + tournament_id())
      .then(response => response.json()
            .then(model => apply_model(model))
            .catch(error => console.log("error in getting model: ", error)))
      .catch(error => console.log("error in request for model: ", error))
  }

  function apply_model(model) {
    
    // console.log("got model: " + JSON.stringify(model))

    next_level_complete_at = model.Transients.EndsAt;
    next_break_at = model.Transients.NextBreakAt;

    var cln = model.State.CurrentLevelNumber;
    var level = model.Structure.Levels[cln]

    if (level.IsBreak) {
      set_html("blinds", level.Description);
      set_class("clock", "clock-break");
    } else {
      set_html("blinds", level.Description);
      set_class("clock", "clock");
    }

    let level_banner = level.Banner;
    if (!model.State.IsClockRunning) {
      level_banner += " (PAUSED)";
    }
    set_html("level", level_banner);

    set_html("current-players", model.State.CurrentPlayers)
    set_html("buyins", model.State.BuyIns)
    // set rebuys
    // set addons
    set_html("avg-chips", model.Transients.AverageChips)
    if (model.Transients.NextLevel !== null) {
      set_html("next-level", model.Transients.NextLevel.Description)
    }
    
    set_clock(model);
    update_clock();
    start_clock();

    maybe_fetch_footers(model.FooterPlugsID);

    last_model = model;
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
  
  function level_remaining() {
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

  function update_break_clock(model) {
    var td = document.getElementById("next-break");
    if (td === null) {
      console.log("update_break_clock: no next-break node to update");
      return;
    }
    
    if (!model.State.IsClockRunning) {
      td.innerHTML = "PAUSED";
      return
    }

    if (!model.Transients.NextBreakAt) {
        td.innerHTML = "N/A";
    } else if (typeof model.Transients.NextBreakAt !== 'number') {
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

  // TODO: ideally, we would not re-sync with the server because we hit end of level;
  // our calculation should be just as good as the server's.  in reality this is
  // unlikely and our clocks are likely to be at least hundreds of milliseconds out
  // of sync in even typical operation.
  function tick() {
    var rem = level_remaining();
    if (typeof rem === 'undefined') {
      // paused, no math to do?
    } else if (rem <= 0) {
      
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
      
      // apply the (possibly fudged) clock
      apply_model(last_model);

      // get the real data from the server to verify the end of level calculation;
      // the local may be different
      load();
    }

    update_clock();
  }

  function set_clock(model) {
    var endsAt = new Date(model.State.CurrentLevelEndsAt)
    next_level_complete_at = endsAt;
  }

  function update_clock() {
    update_break_clock(last_model);
    update_big_clock();
  }

  function update_big_clock() {
    if (typeof next_level_complete_at === 'undefined') {
      // this doesn't happen -- why?
      document.getElementById("clock").innerHTML = "??:??";
    } 
    var render = to_hmmss(level_remaining());
    document.getElementById("clock").innerHTML = render
  }

  function start_reloader() {
    setInterval(load, reload_model_ms);
  }

  var stop_clock = null;
  function start_clock() {
    if (stop_clock === null) {
      var id = setInterval(tick, clock_tick_ms);
      stop_clock = function () {
        clearInterval(id);
      }
    }
  }

  function next_footer_key() {
    next_footer();
    clearInterval(footer_interval_id);
    footer_interval_id = setInterval(next_footer, next_footer_interval_ms);
  }

  function redirect(where) {
    window.location.href = where;
  }

  function send_modify(event) {
    let url = "/api/keyboard-control/" + tournament_id();
    fetch(url, {
      method: 'POST',
      mode: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({"Event":event})
    })
      .then(resp => console.log(`${url} response: ${resp}`))
      .then(_ => load())
      .catch(error => console.log(`error in request for modify event ${event}: ${error}`))
  }

  async function toggle_pause(event) {
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

  function exit_view() {
    redirect('/');
  }

  function redirect_to_edit() {
    redirect(window.location.pathname + "/edit");
  }

  function smwa(arg) {
    return function() { send_modify(arg); }
  }

  // if paused && clock unlocked, send modify with arg
  function ipcusmwa(arg) {
      return function() {
          if (!clock_locked && !last_model.State.IsClockRunning) {
              send_modify(arg);
          } else {
              console.log(`clock_locked=${clock_locked} and running=$(last_model.State.IsClockRunning}`)
          }
      }
  }

  function toggle_clock_lock() {
    clock_locked = !clock_locked;
    if (clock_locked) {
      console.log("clock controls locked");
      set_html("footer", "level/clock controls re-locked");
      start_rotating_footers();
    } else {
      console.log("clock unlocked");
      stop_rotating_footers();
      set_html("footer", "level/clock controls unlocked");
    }
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
      'Backspace': toggle_clock_lock,
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
  
  start_reloader();
  load();
