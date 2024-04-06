// A movement is what makes a clock go.  movement.js makes this poker clock go.

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
  const next_footer_interval_ms = 7500; // 30000
  
  // Initialize last_model (the last model we loaded) with a fail-safe initial
  // model value
  var last_model = {
    "CurrentLevelNumber": 0,
    "Levels": [
      {"TimeRemainingMillis":(59+(56*60))*1000,
       "IsBreak":true,
       "Blinds":"LOADING...",
      },
      {"TimeRemainingMillis":999*60*1000,
       "IsBreak":true,
       "Blinds":"FAKE BREAK LEVEL FOR INIT",
      },
    ],
    "IsClockRunning": false,
    "NextBreakAt": undefined,
    "EndsAt": undefined,
  }

  var footers = ["ATOMIC BATTERIES TO POWER..."];

  const current_level = () => last_model.Levels[last_model.CurrentLevelNumber]

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
    // TODO: hardcoded event id here
    response = fetch("/api/model/1")
      .then(response => response.json()
            .then(model => apply_model(model))
            .catch(error => console.log("error in getting model: ", error)))
      .catch(error => console.log("error in request for model: ", error))
  }

  function apply_model(model) {
    
    console.log("got model: " + JSON.stringify(model))

    next_level_complete_at = model.EndsAt;
    next_break_at = model.NextBreakAt;

    level = model.Levels[model.CurrentLevelNumber]
    var level_banner = "[BANNER UNSET]";
    if (level.IsBreak) {
      if (model.CurrentLevelNumber == 0) {
        level_banner = "STARTING IN...";
      } else {
        level_banner = "BREAK " + model.CurrentLevelNumber;
      }
      set_html("blinds", level.Description);
      set_class("clock", "clock-break");
    } else {
      level_banner = "LEVEL " + model.CurrentLevelNumber;
      set_html("blinds", "BLINDS " + level.Description);
      set_class("clock", "clock");
    }

    if (!model.IsClockRunning) {
      level_banner += " (PAUSED)";
    }
    set_html("level", level_banner);

    set_html("current-players", model.CurrentPlayers)
    set_html("buyins", model.BuyIns)
    // set rebuys
    // set addons
    set_html("avg-chips", model.AverageChips)
    
    set_clock(model);
    update_clock();
    start_clock();

    footers = model.FooterPlugs

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
    var td = document.getElementById("level");
    if (td !== null) {
      td.innerHTML = "LEVEL " + level;
    }
  }

  function level_remaining() {
    ends_at = current_level().EndsAt
    if (ends_at) {
      return ends_at - Date.now();
    } else {
      return current_level().TimeRemainingMillis;
    }
  }

  function update_break_clock(model) {
    var td = document.getElementById("next-break");
    if (td === null) {
      console.log("update_break_clock: no next-break node to update");
      return;
    }
    
    if (!model.IsClockRunning) {
      td.innerHTML = "PAUSED";
      return
    }

    if (typeof model.NextBreakAt === 'undefined') {
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
      last_model.CurrentLevelNumber++
      // apply the model which should move us to the next level
      apply_model(last_model);
      // trust the server as authoritative
      load();
    }

    update_clock();
  }

  function set_clock(model) {
    console.log("Next level complete at " + new Date(model.Levels[model.CurrentLevelNumber].EndsAt));
    next_level_complete_at = model.Levels[model.CurrentLevelNumber].EndsAt;
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

  async function send_modify(event) {
    // TODO: hardcoded event id here
    response = fetch("/api/keyboard-control/1", {
      method: 'POST',
      mode: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({"Event":event})
    })
      .then(resp => console.log("response: " + resp))
      .then(_ => load())
      .catch(error => console.log("error in request for modify event ${event}: ", error))
  }
 
  var keycode_to_handler = {
    'ArrowLeft': { call: send_modify, arg: 'PreviousLevel'},
    'ArrowRight': { call: send_modify, arg: 'SkipLevel'},
    'Space': { call: send_modify, arg: 'TogglePause'},
    'ArrowDown': { call: send_modify, arg: 'MinusMinute'},
    'ArrowUp': { call: send_modify, arg: 'PlusMinute'},
    'PageUp': { call: send_modify, arg: 'AddPlayer'},
    'PageDown': { call: send_modify, arg: 'RemovePlayer'},
    'Home': { call: send_modify, arg: 'AddBuyIn'},
    'End': { call: send_modify, arg: 'RemoveBuyIn'},
  }

  document.addEventListener('keyup', (event) => {
    // var name = event.key;
    var code = event.code;
    // Alert the key name and key code on keydown
    console.log(`Key pressed ${name} \r\n Key code value: ${code}`);
    var handler = keycode_to_handler[code]
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
