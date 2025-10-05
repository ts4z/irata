// A movement is what makes a clock go.  movement.js makes this poker clock go.

function tournament_id() {
  console.log(window.location.pathname);
  var parts = window.location.pathname.split("/");
  id = parts[parts.length - 1];
  console.log("tournament id " + id);
  return id;
}

// t is in milliseconds
function to_hmmss(t) {
  if (isNaN(t)) {
    console.log("can't clock " + t);
    return "♠♠:♠♠";
  }
  if (typeof t === 'undefined') {
    return "♦♦:♦♦";
  }
  if (t < 1000) {
    return "00:00";
  }

  seconds = parseInt(t / 1000)
  var h = parseInt(seconds / 3600)
  var m = parseInt((seconds-(h*3600))/60)
  var s = seconds % 60

  var hh = h
  var mm = m >= 10 ? m : "0" + m
  var ss = s >= 10 ? s : "0" + s

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

(function () {
  const reload_model_ms = 15000;
  const clock_tick_ms = 250;
  const next_footer_interval_ms = 30000

  var next_level_complete_at = undefined;
  
  // Initialize last_model (the last model we loaded) with a fail-safe initial
  // model value
  var last_model = {
    // State is things that are written to the database.
    "State": {
      "CurrentLevelNumber": 0,
      "IsClockRunning": false,
      "TimeRemainingMillis":(59+(59*60))*1000,
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

  var footers = ["ATOMIC BATTERIES TO POWER..."];
  var fetched_footer_plugs_id = NaN;

  async function maybe_fetch_footers(footer_plugs_id) {
    if (footer_plugs_id === fetched_footer_plugs_id) {
      return
    }
    response = fetch("/api/footerPlugs/" + footer_plugs_id)
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
      next_footer_offset++;
      if (next_footer_offset > footers.length) {
        next_footer_offset = 0;
        shuffle_array(footers);
      }
      set_html("footer", footers[next_footer_offset % footers.length]);
    }
  })()

  var footer_interval_id = setInterval(next_footer, next_footer_interval_ms);
  
  async function load() {
    response = fetch("/api/model/" + tournament_id())
      .then(response => response.json()
            .then(model => apply_model(model))
            .catch(error => console.log("error in getting model: ", error)))
      .catch(error => console.log("error in request for model: ", error))
  }

  function apply_model(model) {
    
    console.log("got model: " + JSON.stringify(model))

    next_level_complete_at = model.Transients.EndsAt;
    next_break_at = model.Transients.NextBreakAt;

    var cln = model.State.CurrentLevelNumber;
    level = model.Structure.Levels[cln]
    var level_banner = "[BANNER UNSET]";
    if (level.IsBreak) {
      if (model.State.CurrentLevelNumber == 0) {
        level_banner = "STARTING IN...";
      } else {
        level_banner = "BREAK " + cln;
      }
      set_html("blinds", level.Description);
      set_class("clock", "clock-break");
    } else {
      level_banner = "LEVEL " + cln;
      set_html("blinds", "BLINDS " + level.Description);
      set_class("clock", "clock");
    }

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
    var el = document.getElementById(id)
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
  
  function update_level() {
    {
      var td = document.getElementById("level");
      if (td !== null) {
        td.innerHTML = "LEVEL " + level;
      }
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

    if (typeof model.Transients.NextBreakAt === 'undefined') {
      console.log("update_break_clock: NextBreakAt not defined");
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

  function tick() {
    var rem = level_remaining();
    if (typeof rem === 'undefined') {
      // paused, no math to do?
    } else if (rem <= 0) {
      console.Log("rem = " + rem);
      last_model.State.CurrentLevelNumber++
      // apply the model which should move us to the next level
      apply_model(last_model);
      // trust the server as authoritative
      return load();
    }

    update_clock();
  }

  function set_clock(model) {
    var endsAt = new Date(model.State.CurrentLevelEndsAt)
    console.log("This level complete at " + endsAt)
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

  function send_modify(event) {
    url = "/api/keyboard-control/" + tournament_id();
    response = fetch(url, {
      method: 'POST',
      mode: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({"Event":event})
    })
      .then(resp => console.log("${url} response: " + resp))
      .then(_ => load())
      .catch(error => console.log("error in request for modify event ${event}: ", error))
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
 
  var keycode_to_handler = {
    'ArrowLeft': { call: send_modify, arg: 'PreviousLevel'},
    'ArrowRight': { call: send_modify, arg: 'SkipLevel'},
    'Space': { call: toggle_pause, arg: 'N/A'},
    'ArrowDown': { call: send_modify, arg: 'MinusMinute'},
    'ArrowUp': { call: send_modify, arg: 'PlusMinute'},
    'PageUp': { call: send_modify, arg: 'AddPlayer'},
    'PageDown': { call: send_modify, arg: 'RemovePlayer'},
    'Home': { call: send_modify, arg: 'AddBuyIn'},
    'End': { call: send_modify, arg: 'RemoveBuyIn'},
    'Equal': { call: send_modify, arg: 'AddBuyIn'},
    'Minus': { call: send_modify, arg: 'RemoveBuyIn'},
    'KeyG': { call: send_modify, arg: 'StartClock'},
    'KeyS': { call: send_modify, arg: 'StopClock'},
  }

  document.addEventListener('keyup', (event) => {
    var code = event.code;
    console.log(`Key pressed, Key code value: ${code}`);
    var handler = keycode_to_handler[code];
    if (typeof handler !== 'undefined') {
      console.log(`Key pressed ${event} ${code} => ${handler}`);
      handler.call(handler.arg);
    } else {
      console.log(`drop key ${code}`)
    }
  }, false);
  
  start_reloader();
  load();
})();
